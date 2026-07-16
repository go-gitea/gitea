// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	packages_model "gitea.dev/models/packages"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/convert"

	"gitea.com/gitea/runner/act/model"
)

type methodCtxKeyType struct{}

var methodCtxKey methodCtxKeyType

// withMethod sets the notification method that this context currently executes.
// Used for debugging/ troubleshooting purposes.
func withMethod(ctx context.Context, method string) context.Context {
	// don't overwrite
	if v := ctx.Value(methodCtxKey); v != nil {
		if _, ok := v.(string); ok {
			return ctx
		}
	}
	return context.WithValue(ctx, methodCtxKey, method)
}

// getMethod gets the notification method that this context currently executes.
// Default: "notify"
// Used for debugging/ troubleshooting purposes.
func getMethod(ctx context.Context) string {
	if v := ctx.Value(methodCtxKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "notify"
}

type notifyInput struct {
	// required
	Repo  *repo_model.Repository
	Doer  *user_model.User
	Event webhook_module.HookEventType

	// optional
	Ref         git.RefName
	Payload     api.Payloader
	PullRequest *issues_model.PullRequest
}

func newNotifyInput(repo *repo_model.Repository, doer *user_model.User, event webhook_module.HookEventType) *notifyInput {
	return &notifyInput{
		Repo:  repo,
		Doer:  doer,
		Event: event,
	}
}

func newNotifyInputForSchedules(repo *repo_model.Repository) *notifyInput {
	// the doer here will be ignored as we force using action user when handling schedules
	return newNotifyInput(repo, user_model.NewActionsUser(), webhook_module.HookEventSchedule)
}

func (input *notifyInput) WithDoer(doer *user_model.User) *notifyInput {
	input.Doer = doer
	return input
}

func (input *notifyInput) WithRef(ref string) *notifyInput {
	input.Ref = git.RefName(ref)
	return input
}

func (input *notifyInput) WithPayload(payload api.Payloader) *notifyInput {
	input.Payload = payload
	return input
}

func (input *notifyInput) WithPullRequest(pr *issues_model.PullRequest) *notifyInput {
	input.PullRequest = pr
	if input.Ref == "" {
		input.Ref = git.RefName(pr.GetGitHeadRefName())
	}
	return input
}

func (input *notifyInput) Notify(ctx context.Context) {
	log.Trace("execute %v for event %v whose doer is %v", getMethod(ctx), input.Event, input.Doer.Name)

	if err := notify(ctx, input); err != nil {
		log.Error("an error occurred while executing the %s actions method: %v", getMethod(ctx), err)
	}
}

func notify(ctx context.Context, input *notifyInput) error {
	shouldDetectSchedules := input.Event == webhook_module.HookEventPush && input.Ref.BranchName() == input.Repo.DefaultBranch
	if input.Doer.IsGiteaActions() {
		// avoiding triggering cyclically, for example:
		// a comment of an issue will trigger the runner to add a new comment as reply,
		// and the new comment will trigger the runner again.
		log.Debug("ignore executing %v for event %v whose doer is %v", getMethod(ctx), input.Event, input.Doer.Name)

		// we should update schedule tasks in this case, because
		//   1. schedule tasks cannot be triggered by other events, so cyclic triggering will not occur
		//   2. some schedule tasks may update the repo periodically, so the refs of schedule tasks need to be updated
		if shouldDetectSchedules {
			return DetectAndHandleSchedules(ctx, input.Repo)
		}

		return nil
	}
	if input.Repo.IsEmpty || input.Repo.IsArchived {
		return nil
	}
	if unit_model.TypeActions.UnitGlobalDisabled() {
		if err := CleanRepoScheduleTasks(ctx, input.Repo); err != nil {
			log.Error("CleanRepoScheduleTasks: %v", err)
		}
		return nil
	}
	if err := input.Repo.LoadUnits(ctx); err != nil {
		return fmt.Errorf("repo.LoadUnits: %w", err)
	} else if !input.Repo.UnitEnabled(ctx, unit_model.TypeActions) {
		return nil
	}

	gitRepo, err := gitrepo.OpenRepository(context.Background(), input.Repo)
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	ref := input.Ref
	if ref.BranchName() != input.Repo.DefaultBranch && actions_module.IsDefaultBranchWorkflow(input.Event) {
		if ref != "" {
			log.Warn("Event %q should only trigger workflows on the default branch, but its ref is %q. Will fall back to the default branch",
				input.Event, ref)
		}
		ref = git.RefNameFromBranch(input.Repo.DefaultBranch)
	}
	if ref == "" {
		log.Warn("Ref of event %q is empty, will fall back to the default branch", input.Event)
		ref = git.RefNameFromBranch(input.Repo.DefaultBranch)
	}

	commitID, err := gitRepo.GetRefCommitID(ref.String())
	if err != nil {
		return fmt.Errorf("gitRepo.GetRefCommitID: %w", err)
	}

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %w", err)
	}

	if skipWorkflows(ctx, input, commit) {
		return nil
	}

	var detectedWorkflows []*actions_module.DetectedWorkflow
	var filteredWorkflows []*actions_module.DetectedWorkflow
	actionsConfig := input.Repo.MustGetUnit(ctx, unit_model.TypeActions).ActionsConfig()
	workflows, schedules, filtered, err := actions_module.DetectWorkflows(ctx, gitRepo, commit,
		input.Event,
		input.Payload,
		shouldDetectSchedules,
	)
	if err != nil {
		return fmt.Errorf("DetectWorkflows: %w", err)
	}

	log.Trace("repo %s with commit %s event %s find %d workflows and %d schedules",
		input.Repo.RelativePath(),
		commit.ID,
		input.Event,
		len(workflows),
		len(schedules),
	)

	for _, wf := range workflows {
		if actionsConfig.IsWorkflowDisabled(wf.EntryName) {
			log.Trace("repo %s has disable workflows %s", input.Repo.RelativePath(), wf.EntryName)
			continue
		}

		if wf.TriggerEvent.Name != actions_module.GithubEventPullRequestTarget {
			detectedWorkflows = append(detectedWorkflows, wf)
		}
	}

	for _, wf := range filtered {
		if actionsConfig.IsWorkflowDisabled(wf.EntryName) {
			log.Trace("repo %s has disable workflows %s", input.Repo.RelativePath(), wf.EntryName)
			continue
		}

		if wf.TriggerEvent.Name != actions_module.GithubEventPullRequestTarget {
			filteredWorkflows = append(filteredWorkflows, wf)
		}
	}

	if input.PullRequest != nil {
		// detect pull_request_target workflows
		baseRef := git.BranchPrefix + input.PullRequest.BaseBranch
		baseCommit, err := gitRepo.GetCommit(baseRef)
		if err != nil {
			return fmt.Errorf("gitRepo.GetCommit: %w", err)
		}
		baseWorkflows, _, baseFiltered, err := actions_module.DetectWorkflows(ctx, gitRepo, baseCommit, input.Event, input.Payload, false)
		if err != nil {
			return fmt.Errorf("DetectWorkflows: %w", err)
		}
		if len(baseWorkflows) == 0 {
			log.Trace("repo %s with commit %s couldn't find pull_request_target workflows", input.Repo.RelativePath(), baseCommit.ID)
		} else {
			for _, wf := range baseWorkflows {
				if actionsConfig.IsWorkflowDisabled(wf.EntryName) {
					log.Trace("repo %s has disable workflows %s", input.Repo.RelativePath(), wf.EntryName)
					continue
				}
				if wf.TriggerEvent.Name == actions_module.GithubEventPullRequestTarget {
					detectedWorkflows = append(detectedWorkflows, wf)
				}
			}
		}
		for _, wf := range baseFiltered {
			if actionsConfig.IsWorkflowDisabled(wf.EntryName) {
				log.Trace("repo %s has disable workflows %s", input.Repo.RelativePath(), wf.EntryName)
				continue
			}
			if wf.TriggerEvent.Name == actions_module.GithubEventPullRequestTarget {
				filteredWorkflows = append(filteredWorkflows, wf)
			}
		}
	}

	if shouldDetectSchedules {
		if err := handleSchedules(ctx, schedules, commit, input, ref); err != nil {
			return err
		}
	}

	if err := handleWorkflows(ctx, detectedWorkflows, commit, input, ref); err != nil {
		return err
	}

	handleFilteredWorkflows(ctx, input, filteredWorkflows)

	return detectAndHandleScopedWorkflows(ctx, input, ref, gitRepo, commit)
}

func skipWorkflows(ctx context.Context, input *notifyInput, commit *git.Commit) bool {
	// skip workflow runs with a configured skip-ci string in commit message or pr title if the event is push or pull_request(_sync)
	// https://docs.github.com/en/actions/managing-workflow-runs/skipping-workflow-runs
	skipWorkflowEvents := []webhook_module.HookEventType{
		webhook_module.HookEventPush,
		webhook_module.HookEventPullRequest,
		webhook_module.HookEventPullRequestSync,
	}
	if slices.Contains(skipWorkflowEvents, input.Event) {
		for _, s := range setting.Actions.SkipWorkflowStrings {
			if input.PullRequest != nil && strings.Contains(input.PullRequest.Issue.Title, s) {
				log.Debug("repo %s: skipped run for pr %v because of %s string", input.Repo.RelativePath(), input.PullRequest.Issue.ID, s)
				return true
			}
			if strings.Contains(commit.MessageRaw, s) {
				log.Debug("repo %s with commit %s: skipped run because of %s string", input.Repo.RelativePath(), commit.ID, s)
				return true
			}
		}
	}
	if input.Event == webhook_module.HookEventWorkflowRun {
		wrun, ok := input.Payload.(*api.WorkflowRunPayload)
		for i := 0; i < 5 && ok && wrun.WorkflowRun != nil; i++ {
			if wrun.WorkflowRun.Event != "workflow_run" {
				return false
			}
			r, err := actions_model.GetRunByRepoAndID(ctx, input.Repo.ID, wrun.WorkflowRun.ID)
			if err != nil {
				log.Error("GetRunByRepoAndID: %v", err)
				return true
			}
			wrun, err = r.GetWorkflowRunEventPayload()
			if err != nil {
				log.Error("GetWorkflowRunEventPayload: %v", err)
				return true
			}
		}
		// skip workflow runs events exceeding the maximum of 5 recursive events
		log.Debug("repo %s: skipped workflow_run because of recursive event of 5", input.Repo.RelativePath())
		return true
	}
	return false
}

func handleWorkflows(
	ctx context.Context,
	detectedWorkflows []*actions_module.DetectedWorkflow,
	commit *git.Commit,
	input *notifyInput,
	ref git.RefName,
) error {
	if len(detectedWorkflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", input.Repo.RelativePath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	isForkPullRequest := isForkPullRequestInput(input)

	for _, dwf := range detectedWorkflows {
		// repo-level run: the workflow content is this repo at this commit
		if err := buildApproveAndInsertRun(ctx, input, ref, commit, string(p), isForkPullRequest, dwf, input.Repo.ID, commit.ID.String(), false); err != nil {
			log.Error("repo %s: %v", input.Repo.RelativePath(), err)
			continue
		}
	}
	return nil
}

// buildApproveAndInsertRun assembles an ActionRun for a detected workflow, runs the
// fork-PR approval gate, and inserts it. Repo-level and scoped runs share this path so
// run construction and the approval flow have a single implementation that can't drift.
// workflowRepoID/workflowCommitSHA point at the repo+commit the workflow content comes
// from (the repo itself for repo-level runs, the source repo for scoped runs).
func buildApproveAndInsertRun(
	ctx context.Context,
	input *notifyInput,
	ref git.RefName,
	commit *git.Commit,
	payload string,
	isForkPullRequest bool,
	dwf *actions_module.DetectedWorkflow,
	workflowRepoID int64,
	workflowCommitSHA string,
	isScopedRun bool,
) error {
	run := &actions_model.ActionRun{
		Title:             commit.MessageTitle(),
		RepoID:            input.Repo.ID,
		Repo:              input.Repo,
		OwnerID:           input.Repo.OwnerID,
		WorkflowID:        dwf.EntryName,
		TriggerUserID:     input.Doer.ID,
		TriggerUser:       input.Doer,
		Ref:               ref.String(),
		CommitSHA:         commit.ID.String(),
		IsForkPullRequest: isForkPullRequest,
		Event:             input.Event,
		EventPayload:      payload,
		TriggerEvent:      dwf.TriggerEvent.Name,
		Status:            actions_model.StatusWaiting,
		WorkflowRepoID:    workflowRepoID,
		WorkflowCommitSHA: workflowCommitSHA,
		IsScopedRun:       isScopedRun,
	}

	need, err := ifNeedApproval(ctx, run, input.Repo, input.Doer)
	if err != nil {
		return fmt.Errorf("check if need approval for user %d: %w", input.Doer.ID, err)
	}
	run.NeedApproval = need

	if err := PrepareRunAndInsert(ctx, dwf.Content, run, nil); err != nil {
		return fmt.Errorf("PrepareRunAndInsert: %w", err)
	}
	return nil
}

// handleFilteredWorkflows posts a skipped commit status for each filtered-out workflow whose context is a required status check;
// a non-required one posts nothing, so it cannot leak into a pull request.
func handleFilteredWorkflows(ctx context.Context, input *notifyInput, filteredWorkflows []*actions_module.DetectedWorkflow) {
	if len(filteredWorkflows) == 0 {
		return
	}
	requiredGlobs, err := getAllRequiredStatusContextGlobs(ctx, input.Repo)
	if err != nil {
		log.Error("repo %s: required status contexts: %v", input.Repo.RelativePath(), err)
		return
	}
	if len(requiredGlobs) == 0 {
		return
	}
	for _, dwf := range filteredWorkflows {
		if err := CreateSkippedCommitStatusForFilteredWorkflow(ctx, input.Repo, input.Event, dwf.TriggerEvent.Name, dwf.EntryName, dwf.Content, input.Payload, "", requiredGlobs); err != nil {
			log.Error("repo %s: skipped commit status for workflow %s: %v", input.Repo.RelativePath(), dwf.EntryName, err)
			continue
		}
	}
}

func newNotifyInputFromIssue(issue *issues_model.Issue, event webhook_module.HookEventType) *notifyInput {
	return newNotifyInput(issue.Repo, issue.Poster, event)
}

func notifyRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	permission, _ := access_model.GetDoerRepoPermission(ctx, rel.Repo, doer)

	newNotifyInput(rel.Repo, doer, webhook_module.HookEventRelease).
		WithRef(git.RefNameFromTag(rel.TagName).String()).
		WithPayload(&api.ReleasePayload{
			Action:     action,
			Release:    convert.ToAPIRelease(ctx, rel.Repo, rel),
			Repository: convert.ToRepo(ctx, rel.Repo, permission),
			Sender:     convert.ToUser(ctx, doer, nil),
		}).
		Notify(ctx)
}

func notifyPackage(ctx context.Context, sender *user_model.User, pd *packages_model.PackageDescriptor, action api.HookPackageAction) {
	if pd.Repository == nil {
		// When a package is uploaded to an organization, it could trigger an event to notify.
		// So the repository could be nil, however, actions can't support that yet.
		// See https://github.com/go-gitea/gitea/pull/17940
		return
	}

	apiPackage, err := convert.ToPackage(ctx, pd, sender)
	if err != nil {
		log.Error("Error converting package: %v", err)
		return
	}

	newNotifyInput(pd.Repository, sender, webhook_module.HookEventPackage).
		WithPayload(&api.PackagePayload{
			Action:  action,
			Package: apiPackage,
			Sender:  convert.ToUser(ctx, sender, nil),
		}).
		Notify(ctx)
}

func ifNeedApproval(ctx context.Context, run *actions_model.ActionRun, repo *repo_model.Repository, user *user_model.User) (bool, error) {
	canWrite := func(ctx context.Context, repo *repo_model.Repository, user *user_model.User) (bool, error) {
		perm, err := access_model.GetDoerRepoPermission(ctx, repo, user)
		if err != nil {
			return false, err
		}
		return perm.CanWrite(unit_model.TypeActions), nil
	}
	return ifNeedApprovalWith(ctx, run, repo, user, canWrite, issues_model.HasMergedPullRequestInRepo)
}

func ifNeedApprovalWith(
	ctx context.Context,
	run *actions_model.ActionRun,
	repo *repo_model.Repository,
	user *user_model.User,
	canWriteActions func(context.Context, *repo_model.Repository, *user_model.User) (bool, error),
	hasMergedPR func(context.Context, int64, int64) (bool, error),
) (bool, error) {
	// 1. don't need approval if it's not a fork PR
	// 2. don't need approval if the event is `pull_request_target` since the workflow will run in the context of base branch
	// 		see https://docs.github.com/en/actions/managing-workflow-runs/approving-workflow-runs-from-public-forks#about-workflow-runs-from-public-forks
	if !run.IsForkPullRequest || run.TriggerEvent == actions_module.GithubEventPullRequestTarget {
		return false, nil
	}

	// always need approval if the user is restricted
	if user.IsRestricted {
		log.Trace("need approval because user %d is restricted", user.ID)
		return true, nil
	}

	// don't need approval if the user can write
	if ok, err := canWriteActions(ctx, repo, user); err != nil {
		return false, fmt.Errorf("GetDoerRepoPermission: %w", err)
	} else if ok {
		log.Trace("do not need approval because user %d can write", user.ID)
		return false, nil
	}

	// trust the user only after a merged PR — matching GitHub Actions. Approving one
	// fork PR's run must not implicitly trust later fork PRs that replace the workflow.
	if merged, err := hasMergedPR(ctx, repo.ID, user.ID); err != nil {
		return false, fmt.Errorf("HasMergedPullRequestInRepo: %w", err)
	} else if merged {
		log.Trace("do not need approval because user %d has a merged pull request in repo %d", user.ID, repo.ID)
		return false, nil
	}

	// otherwise, need approval
	log.Trace("need approval because user %d has no merged pull request in repo %d", user.ID, repo.ID)
	return true, nil
}

func handleSchedules(
	ctx context.Context,
	detectedWorkflows []*actions_module.DetectedWorkflow,
	commit *git.Commit,
	input *notifyInput,
	ref git.RefName,
) error {
	if ref.BranchName() != input.Repo.DefaultBranch {
		log.Trace("commit branch is not default branch in repo")
		return nil
	}

	if count, err := db.Count[actions_model.ActionSchedule](ctx, actions_model.FindScheduleOptions{RepoID: input.Repo.ID}); err != nil {
		log.Error("CountSchedules: %v", err)
		return err
	} else if count > 0 {
		if err := CleanRepoScheduleTasks(ctx, input.Repo); err != nil {
			log.Error("CleanRepoScheduleTasks: %v", err)
		}
	}

	if len(detectedWorkflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find schedules", input.Repo.RelativePath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	crons := make([]*actions_model.ActionSchedule, 0, len(detectedWorkflows))
	for _, dwf := range detectedWorkflows {
		// Check cron job condition. Only working in default branch
		workflow, err := model.ReadWorkflow(bytes.NewReader(dwf.Content))
		if err != nil {
			log.Error("ReadWorkflow: %v", err)
			continue
		}
		schedules := workflow.OnSchedule()
		if len(schedules) == 0 {
			log.Warn("no schedule event")
			continue
		}

		run := &actions_model.ActionSchedule{
			Title:         commit.MessageTitle(),
			RepoID:        input.Repo.ID,
			Repo:          input.Repo,
			OwnerID:       input.Repo.OwnerID,
			WorkflowID:    dwf.EntryName,
			TriggerUserID: user_model.ActionsUserID,
			TriggerUser:   user_model.NewActionsUser(),
			Ref:           ref.String(),
			CommitSHA:     commit.ID.String(),
			Event:         input.Event,
			EventPayload:  string(p),
			Specs:         schedules,
			Content:       dwf.Content,
		}

		crons = append(crons, run)
	}

	return actions_model.CreateScheduleTask(ctx, crons)
}

// DetectAndHandleSchedules detects the schedule workflows on the default branch and create schedule tasks
func DetectAndHandleSchedules(ctx context.Context, repo *repo_model.Repository) error {
	if repo.IsEmpty || repo.IsArchived {
		return nil
	}

	gitRepo, err := gitrepo.OpenRepository(context.Background(), repo)
	if err != nil {
		return fmt.Errorf("git.OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	// Only detect schedule workflows on the default branch
	commit, err := gitRepo.GetCommit(repo.DefaultBranch)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %w", err)
	}
	scheduleWorkflows, err := actions_module.DetectScheduledWorkflows(ctx, gitRepo, commit)
	if err != nil {
		return fmt.Errorf("detect schedule workflows: %w", err)
	}
	if len(scheduleWorkflows) == 0 {
		return nil
	}

	// We need a notifyInput to call handleSchedules
	// if repo is a mirror, commit author maybe an external user,
	// so we use action user as the Doer of the notifyInput
	notifyInput := newNotifyInputForSchedules(repo)

	return handleSchedules(ctx, scheduleWorkflows, commit, notifyInput, git.RefNameFromBranch(repo.DefaultBranch))
}

// isForkPullRequestInput reports whether the run should be treated as a fork pull request.
func isForkPullRequestInput(input *notifyInput) bool {
	pr := input.PullRequest
	if pr == nil {
		return false
	}
	switch pr.Flow {
	case issues_model.PullRequestFlowGithub:
		return pr.IsFromFork()
	case issues_model.PullRequestFlowAGit:
		// There is no fork concept in agit flow, anyone with read permission can push refs/for/<target-branch>/<topic-branch> to the repo.
		// So we can treat it as a fork pull request because it may be from an untrusted user
		return true
	default:
		// unknown flow, assume it's a fork pull request to be safe
		return true
	}
}

// detectAndHandleScopedWorkflows detects scoped workflows registered for the consuming repo
func detectAndHandleScopedWorkflows(
	ctx context.Context,
	input *notifyInput,
	ref git.RefName,
	consumerGitRepo *git.Repository,
	consumerCommit *git.Commit,
) error {
	// TODO: support workflow_run and schedule
	if input.Event == webhook_module.HookEventWorkflowRun || input.Event == webhook_module.HookEventSchedule {
		return nil
	}

	sources, err := actions_model.GetEffectiveScopedWorkflowSources(ctx, input.Repo.OwnerID)
	if err != nil {
		return fmt.Errorf("GetEffectiveScopedWorkflowSources: %w", err)
	}
	if len(sources) == 0 {
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	isForkPullRequest := isForkPullRequestInput(input)
	actionsConfig := input.Repo.MustGetUnit(ctx, unit_model.TypeActions).ActionsConfig()

	// A filtered-out scoped workflow only posts a skipped status when its context is a required check.
	requiredGlobs, err := getAllRequiredStatusContextGlobs(ctx, input.Repo)
	if err != nil {
		log.Error("scoped workflows: required status contexts for %s: %v", input.Repo.RelativePath(), err)
	}

	// The same source repo may be registered at both the owner and instance level; dedup
	// the IDs and batch-load them in one query instead of one round-trip per source.
	seen := make(container.Set[int64], len(sources))
	for _, source := range sources {
		seen.Add(source.SourceRepoID)
	}
	sourceRepoIDs := seen.Values()

	sourceRepos, err := repo_model.GetRepositoriesMapByIDs(ctx, sourceRepoIDs)
	if err != nil {
		return fmt.Errorf("GetRepositoriesMapByIDs: %w", err)
	}

	for _, sourceRepoID := range sourceRepoIDs {
		sourceRepo := sourceRepos[sourceRepoID]
		if sourceRepo == nil {
			// don't abort the other effective sources for this event
			log.Error("scoped workflows: source repo %d for consumer %s not found", sourceRepoID, input.Repo.RelativePath())
			continue
		}
		if sourceRepo.IsEmpty {
			continue
		}

		sourceCommitSHA, detected, filtered, err := detectScopedWorkflowsForSource(ctx, input, consumerGitRepo, consumerCommit, sourceRepo)
		if err != nil {
			log.Error("scoped workflows: source %d for consumer %s: %v", sourceRepoID, input.Repo.RelativePath(), err)
			continue
		}

		for _, dwf := range detected {
			// A consuming repo can opt out of a non-required scoped workflow.
			// A required workflow (marked required at any effective level) can never be opted out.
			if actions_model.ScopedWorkflowOptedOut(actionsConfig, sources, sourceRepo.ID, dwf.EntryName) {
				continue
			}

			if err := buildApproveAndInsertRun(ctx, input, ref, consumerCommit, string(p), isForkPullRequest, dwf, sourceRepo.ID, sourceCommitSHA, true); err != nil {
				log.Error("scoped workflows: source %s workflow %s: %v", sourceRepo.RelativePath(), dwf.EntryName, err)
				continue
			}
		}

		// A filtered-out scoped workflow posts a skipped commit status for its required-check contexts.
		if len(filtered) > 0 && len(requiredGlobs) > 0 {
			scopedPrefix := actions_model.ScopedStatusContextPrefix(ctx, sourceRepo.ID)
			for _, dwf := range filtered {
				if actions_model.ScopedWorkflowOptedOut(actionsConfig, sources, sourceRepo.ID, dwf.EntryName) {
					continue
				}
				if err := CreateSkippedCommitStatusForFilteredWorkflow(ctx, input.Repo, input.Event, dwf.TriggerEvent.Name, dwf.EntryName, dwf.Content, input.Payload, scopedPrefix, requiredGlobs); err != nil {
					log.Error("scoped workflows: skipped commit status for source %s workflow %s: %v", sourceRepo.RelativePath(), dwf.EntryName, err)
					continue
				}
			}
		}
	}

	return nil
}

// detectScopedWorkflowsForSource detects the scoped workflows from the source repo at its default branch.
// detected are the workflows to run; filtered matched the event but were excluded by a branch/paths
// filter, and later post a skipped commit status only for a required-check context.
func detectScopedWorkflowsForSource(
	ctx context.Context,
	input *notifyInput,
	consumerGitRepo *git.Repository,
	consumerCommit *git.Commit,
	sourceRepo *repo_model.Repository,
) (sourceCommitSHA string, detected, filtered []*actions_module.DetectedWorkflow, err error) {
	// scoped workflow content is always taken from the source repo's default branch; the parse is cached per (source, default-branch SHA) and reused across consuming repos/events

	sourceCommitSHA, parsed, err := LoadParsedScopedWorkflows(ctx, sourceRepo)
	if err != nil {
		return "", nil, nil, err
	}
	detected, filtered = actions_module.MatchScopedWorkflows(parsed, consumerGitRepo, consumerCommit, input.Event, input.Payload)
	return sourceCommitSHA, detected, filtered, nil
}
