// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"

	"github.com/nektos/act/pkg/jobparser"
	"github.com/nektos/act/pkg/model"
)

var methodCtxKey struct{}

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
	Ref         string
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
	input.Ref = ref
	return input
}

func (input *notifyInput) WithPayload(payload api.Payloader) *notifyInput {
	input.Payload = payload
	return input
}

func (input *notifyInput) WithPullRequest(pr *issues_model.PullRequest) *notifyInput {
	input.PullRequest = pr
	if input.Ref == "" {
		input.Ref = pr.GetGitRefName()
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
	if input.Doer.IsActions() {
		// avoiding triggering cyclically, for example:
		// a comment of an issue will trigger the runner to add a new comment as reply,
		// and the new comment will trigger the runner again.
		log.Debug("ignore executing %v for event %v whose doer is %v", getMethod(ctx), input.Event, input.Doer.Name)
		return nil
	}
	if input.Repo.IsEmpty || input.Repo.IsArchived {
		return nil
	}
	if unit_model.TypeActions.UnitGlobalDisabled() {
		if err := actions_model.CleanRepoScheduleTasks(ctx, input.Repo); err != nil {
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
	if ref != input.Repo.DefaultBranch && actions_module.IsDefaultBranchWorkflow(input.Event) {
		if ref != "" {
			log.Warn("Event %q should only trigger workflows on the default branch, but its ref is %q. Will fall back to the default branch",
				input.Event, ref)
		}
		ref = input.Repo.DefaultBranch
	}
	if ref == "" {
		log.Warn("Ref of event %q is empty, will fall back to the default branch", input.Event)
		ref = input.Repo.DefaultBranch
	}

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return fmt.Errorf("gitRepo.GetCommit: %w", err)
	}

	if skipWorkflows(input, commit) {
		return nil
	}

	var detectedWorkflows []*actions_module.DetectedWorkflow
	actionsConfig := input.Repo.MustGetUnit(ctx, unit_model.TypeActions).ActionsConfig()
	shouldDetectSchedules := input.Event == webhook_module.HookEventPush && git.RefName(input.Ref).BranchName() == input.Repo.DefaultBranch
	workflows, schedules, err := actions_module.DetectWorkflows(gitRepo, commit,
		input.Event,
		input.Payload,
		shouldDetectSchedules,
	)
	if err != nil {
		return fmt.Errorf("DetectWorkflows: %w", err)
	}

	log.Trace("repo %s with commit %s event %s find %d workflows and %d schedules",
		input.Repo.RepoPath(),
		commit.ID,
		input.Event,
		len(workflows),
		len(schedules),
	)

	for _, wf := range workflows {
		if actionsConfig.IsWorkflowDisabled(wf.EntryName) {
			log.Trace("repo %s has disable workflows %s", input.Repo.RepoPath(), wf.EntryName)
			continue
		}

		if wf.TriggerEvent.Name != actions_module.GithubEventPullRequestTarget {
			detectedWorkflows = append(detectedWorkflows, wf)
		}
	}

	if input.PullRequest != nil {
		// detect pull_request_target workflows
		baseRef := git.BranchPrefix + input.PullRequest.BaseBranch
		baseCommit, err := gitRepo.GetCommit(baseRef)
		if err != nil {
			return fmt.Errorf("gitRepo.GetCommit: %w", err)
		}
		baseWorkflows, _, err := actions_module.DetectWorkflows(gitRepo, baseCommit, input.Event, input.Payload, false)
		if err != nil {
			return fmt.Errorf("DetectWorkflows: %w", err)
		}
		if len(baseWorkflows) == 0 {
			log.Trace("repo %s with commit %s couldn't find pull_request_target workflows", input.Repo.RepoPath(), baseCommit.ID)
		} else {
			for _, wf := range baseWorkflows {
				if wf.TriggerEvent.Name == actions_module.GithubEventPullRequestTarget {
					detectedWorkflows = append(detectedWorkflows, wf)
				}
			}
		}
	}

	if shouldDetectSchedules {
		if err := handleSchedules(ctx, schedules, commit, input, ref); err != nil {
			return err
		}
	}

	return handleWorkflows(ctx, detectedWorkflows, commit, input, ref)
}

func skipWorkflows(input *notifyInput, commit *git.Commit) bool {
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
				log.Debug("repo %s: skipped run for pr %v because of %s string", input.Repo.RepoPath(), input.PullRequest.Issue.ID, s)
				return true
			}
			if strings.Contains(commit.CommitMessage, s) {
				log.Debug("repo %s with commit %s: skipped run because of %s string", input.Repo.RepoPath(), commit.ID, s)
				return true
			}
		}
	}
	return false
}

func handleWorkflows(
	ctx context.Context,
	detectedWorkflows []*actions_module.DetectedWorkflow,
	commit *git.Commit,
	input *notifyInput,
	ref string,
) error {
	if len(detectedWorkflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find workflows", input.Repo.RepoPath(), commit.ID)
		return nil
	}

	p, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	isForkPullRequest := false
	if pr := input.PullRequest; pr != nil {
		switch pr.Flow {
		case issues_model.PullRequestFlowGithub:
			isForkPullRequest = pr.IsFromFork()
		case issues_model.PullRequestFlowAGit:
			// There is no fork concept in agit flow, anyone with read permission can push refs/for/<target-branch>/<topic-branch> to the repo.
			// So we can treat it as a fork pull request because it may be from an untrusted user
			isForkPullRequest = true
		default:
			// unknown flow, assume it's a fork pull request to be safe
			isForkPullRequest = true
		}
	}

	for _, dwf := range detectedWorkflows {
		run := &actions_model.ActionRun{
			Title:             strings.SplitN(commit.CommitMessage, "\n", 2)[0],
			RepoID:            input.Repo.ID,
			OwnerID:           input.Repo.OwnerID,
			WorkflowID:        dwf.EntryName,
			TriggerUserID:     input.Doer.ID,
			Ref:               ref,
			CommitSHA:         commit.ID.String(),
			IsForkPullRequest: isForkPullRequest,
			Event:             input.Event,
			EventPayload:      string(p),
			TriggerEvent:      dwf.TriggerEvent.Name,
			Status:            actions_model.StatusWaiting,
		}

		need, err := ifNeedApproval(ctx, run, input.Repo, input.Doer)
		if err != nil {
			log.Error("check if need approval for repo %d with user %d: %v", input.Repo.ID, input.Doer.ID, err)
			continue
		}

		run.NeedApproval = need

		if err := run.LoadAttributes(ctx); err != nil {
			log.Error("LoadAttributes: %v", err)
			continue
		}

		vars, err := actions_model.GetVariablesOfRun(ctx, run)
		if err != nil {
			log.Error("GetVariablesOfRun: %v", err)
			continue
		}

		jobs, err := jobparser.Parse(dwf.Content, jobparser.WithVars(vars))
		if err != nil {
			log.Error("jobparser.Parse: %v", err)
			continue
		}

		// cancel running jobs if the event is push or pull_request_sync
		if run.Event == webhook_module.HookEventPush ||
			run.Event == webhook_module.HookEventPullRequestSync {
			if err := actions_model.CancelPreviousJobs(
				ctx,
				run.RepoID,
				run.Ref,
				run.WorkflowID,
				run.Event,
			); err != nil {
				log.Error("CancelPreviousJobs: %v", err)
			}
		}

		if err := actions_model.InsertRun(ctx, run, jobs); err != nil {
			log.Error("InsertRun: %v", err)
			continue
		}

		alljobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
		if err != nil {
			log.Error("FindRunJobs: %v", err)
			continue
		}
		CreateCommitStatus(ctx, alljobs...)
	}
	return nil
}

func newNotifyInputFromIssue(issue *issues_model.Issue, event webhook_module.HookEventType) *notifyInput {
	return newNotifyInput(issue.Repo, issue.Poster, event)
}

func notifyRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	permission, _ := access_model.GetUserRepoPermission(ctx, rel.Repo, doer)

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
	if perm, err := access_model.GetUserRepoPermission(ctx, repo, user); err != nil {
		return false, fmt.Errorf("GetUserRepoPermission: %w", err)
	} else if perm.CanWrite(unit_model.TypeActions) {
		log.Trace("do not need approval because user %d can write", user.ID)
		return false, nil
	}

	// don't need approval if the user has been approved before
	if count, err := db.Count[actions_model.ActionRun](ctx, actions_model.FindRunOptions{
		RepoID:        repo.ID,
		TriggerUserID: user.ID,
		Approved:      true,
	}); err != nil {
		return false, fmt.Errorf("CountRuns: %w", err)
	} else if count > 0 {
		log.Trace("do not need approval because user %d has been approved before", user.ID)
		return false, nil
	}

	// otherwise, need approval
	log.Trace("need approval because it's the first time user %d triggered actions", user.ID)
	return true, nil
}

func handleSchedules(
	ctx context.Context,
	detectedWorkflows []*actions_module.DetectedWorkflow,
	commit *git.Commit,
	input *notifyInput,
	ref string,
) error {
	branch, err := commit.GetBranchName()
	if err != nil {
		return err
	}
	if branch != input.Repo.DefaultBranch {
		log.Trace("commit branch is not default branch in repo")
		return nil
	}

	if count, err := db.Count[actions_model.ActionSchedule](ctx, actions_model.FindScheduleOptions{RepoID: input.Repo.ID}); err != nil {
		log.Error("CountSchedules: %v", err)
		return err
	} else if count > 0 {
		if err := actions_model.CleanRepoScheduleTasks(ctx, input.Repo); err != nil {
			log.Error("CleanRepoScheduleTasks: %v", err)
		}
	}

	if len(detectedWorkflows) == 0 {
		log.Trace("repo %s with commit %s couldn't find schedules", input.Repo.RepoPath(), commit.ID)
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
			Title:         strings.SplitN(commit.CommitMessage, "\n", 2)[0],
			RepoID:        input.Repo.ID,
			OwnerID:       input.Repo.OwnerID,
			WorkflowID:    dwf.EntryName,
			TriggerUserID: user_model.ActionsUserID,
			Ref:           ref,
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
	scheduleWorkflows, err := actions_module.DetectScheduledWorkflows(gitRepo, commit)
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

	return handleSchedules(ctx, scheduleWorkflows, commit, notifyInput, repo.DefaultBranch)
}
