// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/git"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	webhook_module "gitea.dev/modules/webhook"
)

// ToActionTask convert an actions_model.ActionTask to an api.ActionTask
func ToActionTask(ctx context.Context, t *actions_model.ActionTask) (*api.ActionTask, error) {
	// don't need Steps here, only need to load job and its run
	if err := t.LoadJob(ctx); err != nil {
		return nil, err
	}
	if err := t.Job.LoadRun(ctx); err != nil {
		return nil, err
	}
	if err := t.Job.Run.LoadRepo(ctx); err != nil {
		return nil, err
	}
	return &api.ActionTask{
		ID:           t.ID,
		Name:         t.Job.Name,
		HeadBranch:   t.Job.Run.PrettyRef(),
		HeadSHA:      t.Job.CommitSHA,
		RunNumber:    t.Job.Run.Index,
		Event:        t.Job.Run.TriggerEvent,
		DisplayTitle: t.Job.Run.Title,
		Status:       t.Status.String(),
		WorkflowID:   t.Job.Run.WorkflowID,
		URL:          httplib.MakeAbsoluteURL(ctx, t.Job.Run.Link()),
		CreatedAt:    t.Created.AsLocalTime(),
		UpdatedAt:    t.Updated.AsLocalTime(),
		RunStartedAt: t.Started.AsLocalTime(),
	}, nil
}

func ToActionWorkflowRun(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, excludePullRequests bool) (_ *api.ActionWorkflowRun, err error) {
	if err := run.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	if attempt == nil {
		attempt, _, err = run.GetLatestAttempt(ctx)
		if err != nil {
			return nil, err
		}
	}

	runAttempt := int64(0)
	status, conclusion := ToRunActionsStatus(run, run.Status)
	startedAt := run.Started.AsLocalTime()
	completedAt := run.Stopped.AsLocalTime()
	actor := run.TriggerUser       // The username of the user that triggered the initial workflow run.
	triggerUser := run.TriggerUser // The username of the user that initiated the workflow run. If the workflow run is a re-run, this value may differ from actor.

	// previousAttemptURL is the value of ActionWorkflowRun.PreviousAttemptURL, which is declared as *string without `omitempty` on purpose:
	// a nil value must still appear in the JSON body as `"previous_attempt_url": null`, matching GitHub's Actions API.
	var previousAttemptURL *string

	if attempt != nil {
		attempt.Run = run
		if err := attempt.LoadAttributes(ctx); err != nil {
			return nil, err
		}
		runAttempt = attempt.Attempt
		status, conclusion = ToRunActionsStatus(run, attempt.Status)
		startedAt = attempt.Started.AsLocalTime()
		completedAt = attempt.Stopped.AsLocalTime()
		triggerUser = attempt.TriggerUser
		if attempt.Attempt > 1 {
			url := fmt.Sprintf("%s/attempts/%d", run.APIURL(ctx), attempt.Attempt-1)
			previousAttemptURL = &url
		}
	}
	pullRequests := []*api.PullRequestMinimal{}
	if !excludePullRequests {
		pullRequests, err = loadPullRequestsForRun(ctx, run)
		if err != nil {
			return nil, err
		}
	}

	runURL := run.APIURL(ctx)
	return &api.ActionWorkflowRun{
		ID:                 run.ID,
		URL:                runURL,
		PreviousAttemptURL: previousAttemptURL,
		HTMLURL:            run.HTMLURL(ctx),
		JobsURL:            runURL + "/jobs",
		LogsURL:            runURL + "/logs",
		ArtifactsURL:       runURL + "/artifacts",
		CancelURL:          runURL + "/cancel",
		RerunURL:           runURL + "/rerun",
		RunNumber:          run.Index,
		RunAttempt:         runAttempt,
		CreatedAt:          run.Created.AsLocalTime(),
		UpdatedAt:          run.Updated.AsLocalTime(),
		StartedAt:          startedAt,
		CompletedAt:        completedAt,
		Event:              run.TriggerEvent,
		DisplayTitle:       run.Title,
		HeadBranch:         git.RefName(run.Ref).BranchName(),
		HeadSha:            run.CommitSHA,
		Status:             status,
		Conclusion:         conclusion,
		Path:               fmt.Sprintf("%s@%s", run.WorkflowID, run.Ref),
		Repository:         ToRepo(ctx, run.Repo, access_model.Permission{AccessMode: perm.AccessModeNone}),
		TriggerActor:       ToUser(ctx, triggerUser, nil),
		Actor:              ToUser(ctx, actor, nil),
		PullRequests:       pullRequests,
	}, nil
}

// loadPullRequestsForRun returns the pull requests associated with a run, matching
// GitHub's `pull_requests` field on workflow run responses:
// - For pull_request / pull_request_review events, the PR whose ref triggered the run.
// - For push events, open PRs whose head branch matches the pushed ref in the same repo.
// - For other events, no PRs.
func loadPullRequestsForRun(ctx context.Context, run *actions_model.ActionRun) ([]*api.PullRequestMinimal, error) {
	result := []*api.PullRequestMinimal{}
	refName := git.RefName(run.Ref)
	var prs issues_model.PullRequestList
	switch {
	case run.Event.IsPullRequest() || run.Event.IsPullRequestReview():
		index, ok := refName.PullIndex()
		if !ok {
			return result, nil
		}
		pr, err := issues_model.GetPullRequestByIndex(ctx, run.RepoID, index)
		if err != nil {
			if issues_model.IsErrPullRequestNotExist(err) {
				return result, nil
			}
			return nil, err
		}
		prs = issues_model.PullRequestList{pr}
	case run.Event == webhook_module.HookEventPush:
		branch := refName.BranchName()
		if branch == "" {
			return result, nil
		}
		var err error
		prs, err = issues_model.GetUnmergedPullRequestsByHeadInfo(ctx, run.RepoID, branch)
		if err != nil {
			return nil, err
		}
	default:
		return result, nil
	}
	for _, pr := range prs {
		minimal, err := toPullRequestMinimal(ctx, run.Repo, pr, run.CommitSHA)
		if err != nil {
			return nil, err
		}
		result = append(result, minimal)
	}
	return result, nil
}

func toPullRequestMinimal(ctx context.Context, repo *repo_model.Repository, pr *issues_model.PullRequest, headSHA string) (*api.PullRequestMinimal, error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, err
	}
	headRepo := pr.HeadRepo
	if headRepo == nil {
		headRepo = pr.BaseRepo
	}
	return &api.PullRequestMinimal{
		ID:     pr.ID,
		Number: pr.Index,
		URL:    fmt.Sprintf("%s/pulls/%d", repo.APIURL(ctx), pr.Index),
		Head: api.PullRequestMinimalHead{
			Ref: pr.HeadBranch,
			SHA: headSHA,
			Repo: api.PullRequestMinimalHeadRepo{
				ID:   headRepo.ID,
				URL:  headRepo.APIURL(ctx),
				Name: headRepo.Name,
			},
		},
		Base: api.PullRequestMinimalHead{
			Ref: pr.BaseBranch,
			SHA: pr.MergeBase,
			Repo: api.PullRequestMinimalHeadRepo{
				ID:   pr.BaseRepo.ID,
				URL:  pr.BaseRepo.APIURL(ctx),
				Name: pr.BaseRepo.Name,
			},
		},
	}, nil
}

func ToWorkflowRunAction(status actions_model.Status) (action string) {
	switch status {
	case actions_model.StatusWaiting, actions_model.StatusBlocked:
		action = "requested"
	case actions_model.StatusRunning, actions_model.StatusCancelling:
		action = "in_progress"
	default:
		if status.IsDone() {
			action = "completed"
		} else {
			setting.PanicInDevOrTesting("unknown action status: %v", status)
		}
	}
	return action
}

func ToRunActionsStatus(run *actions_model.ActionRun, status actions_model.Status) (action, conclusion string) {
	if status.IsBlocked() && run.NeedApproval {
		return "waiting", ""
	}
	return ToActionsStatus(status)
}

func ToActionsStatus(status actions_model.Status) (action, conclusion string) {
	switch status {
	case actions_model.StatusWaiting:
		action = "queued"
	case actions_model.StatusBlocked:
		action = "pending"
	case actions_model.StatusPending:
		action = "requested"
	case actions_model.StatusRunning, actions_model.StatusCancelling:
		action = "in_progress"
	default:
		action = "completed"
		switch status {
		case actions_model.StatusSuccess:
			conclusion = "success"
		case actions_model.StatusCancelled:
			conclusion = "cancelled"
		case actions_model.StatusFailure:
			conclusion = "failure"
		case actions_model.StatusSkipped:
			conclusion = "skipped"
		default:
			setting.PanicInDevOrTesting("unknown action status: %v", status)
		}
	}
	return action, conclusion
}

// ToActionWorkflowJob convert a actions_model.ActionRunJob to an api.ActionWorkflowJob
// task is optional and can be nil
func ToActionWorkflowJob(ctx context.Context, repo *repo_model.Repository, task *actions_model.ActionTask, job *actions_model.ActionRunJob) (*api.ActionWorkflowJob, error) {
	err := job.LoadAttributes(ctx)
	if err != nil {
		return nil, err
	}

	status, conclusion := ToRunActionsStatus(job.Run, job.Status)
	var runnerID int64
	var runnerName string
	var steps []*api.ActionWorkflowStep

	if effectiveTaskID := job.EffectiveTaskID(); effectiveTaskID != 0 {
		if task == nil {
			task, _, err = db.GetByID[actions_model.ActionTask](ctx, effectiveTaskID)
			if err != nil {
				return nil, err
			}
		}

		if task != nil {
			if task.Steps == nil {
				task.Steps, err = actions_model.GetTaskStepsByTaskID(ctx, task.ID)
				if err != nil {
					return nil, err
				}
				task.Steps = util.SliceNilAsEmpty(task.Steps)
			}
			runnerID = task.RunnerID
			if runner, ok, _ := db.GetByID[actions_model.ActionRunner](ctx, runnerID); ok {
				runnerName = runner.Name
			}
			for i, step := range task.Steps {
				stepStatus, stepConclusion := ToActionsStatus(step.Status)
				steps = append(steps, &api.ActionWorkflowStep{
					Name:        step.Name,
					Number:      int64(i),
					Status:      stepStatus,
					Conclusion:  stepConclusion,
					StartedAt:   step.Started.AsTime().UTC(),
					CompletedAt: step.Stopped.AsTime().UTC(),
				})
			}
		}
	}

	return &api.ActionWorkflowJob{
		ID: job.ID,
		// missing api endpoint for this location
		URL:     fmt.Sprintf("%s/actions/jobs/%d", repo.APIURL(ctx), job.ID),
		HTMLURL: fmt.Sprintf("%s/jobs/%d", job.Run.HTMLURL(ctx), job.ID),
		RunID:   job.RunID,
		// Missing api endpoint for this location, artifacts are available under a nested url
		RunURL:      fmt.Sprintf("%s/actions/runs/%d", repo.APIURL(ctx), job.RunID),
		Name:        job.Name,
		Labels:      job.RunsOn,
		RunAttempt:  job.Attempt,
		HeadSha:     job.Run.CommitSHA,
		HeadBranch:  git.RefName(job.Run.Ref).BranchName(),
		Status:      status,
		Conclusion:  conclusion,
		RunnerID:    runnerID,
		RunnerName:  runnerName,
		Steps:       util.SliceNilAsEmpty(steps),
		CreatedAt:   job.Created.AsTime().UTC(),
		StartedAt:   job.Started.AsTime().UTC(),
		CompletedAt: job.Stopped.AsTime().UTC(),
	}, nil
}

func getActionWorkflowEntry(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, commit *git.Commit, refName git.RefName, folder string, entry *git.TreeEntry) *api.ActionWorkflow {
	cfgUnit := repo.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()

	workflowURL := fmt.Sprintf("%s/actions/workflows/%s", repo.APIURL(), util.PathEscapeSegments(entry.Name()))
	workflowRepoURL := fmt.Sprintf("%s/src/commit/%s/%s/%s", repo.HTMLURL(ctx), commit.ID.String(), util.PathEscapeSegments(folder), util.PathEscapeSegments(entry.Name()))
	if refWebLinkPath := refName.RefWebLinkPath(); refWebLinkPath != "" {
		workflowRepoURL = fmt.Sprintf("%s/src/%s/%s/%s", repo.HTMLURL(ctx), refWebLinkPath, util.PathEscapeSegments(folder), util.PathEscapeSegments(entry.Name()))
	}
	badgeURL := fmt.Sprintf("%s/actions/workflows/%s/badge.svg?branch=%s", repo.HTMLURL(ctx), util.PathEscapeSegments(entry.Name()), url.QueryEscape(repo.DefaultBranch))

	// See https://docs.github.com/en/rest/actions/workflows?apiVersion=2022-11-28#get-a-workflow
	// State types:
	// - active
	// - deleted
	// - disabled_fork
	// - disabled_inactivity
	// - disabled_manually
	state := "active"
	if cfg.IsWorkflowDisabled(entry.Name()) {
		state = "disabled_manually"
	}

	// The CreatedAt and UpdatedAt fields currently reflect the timestamp of the latest commit, which can later be refined
	// by retrieving the first and last commits for the file history. The first commit would indicate the creation date,
	// while the last commit would represent the modification date. The DeletedAt could be determined by identifying
	// the last commit where the file existed. However, this implementation has not been done here yet, as it would likely
	// cause a significant performance degradation.
	createdAt := commit.Author.When
	updatedAt := commit.Author.When

	content, err := actions.GetContentFromEntry(ctx, gitRepo, entry)
	name := entry.Name()
	if err == nil {
		workflow, err := jobparser.ReadWorkflow(content)
		if err == nil {
			// Only use the name when specified in the workflow file
			if workflow.Name != "" {
				name = workflow.Name
			}
		} else {
			log.Error("getActionWorkflowEntry: Failed to parse workflow: %v", err)
		}
	} else {
		log.Error("getActionWorkflowEntry: Failed to get content from entry: %v", err)
	}

	return &api.ActionWorkflow{
		ID:        entry.Name(),
		Name:      name,
		Path:      path.Join(folder, entry.Name()),
		State:     state,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		URL:       workflowURL,
		HTMLURL:   workflowRepoURL,
		BadgeURL:  badgeURL,
	}
}

func ListActionWorkflows(ctx context.Context, gitrepo *git.Repository, repo *repo_model.Repository) ([]*api.ActionWorkflow, error) {
	defaultBranchCommit, err := gitrepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	folder, entries, err := actions.ListWorkflows(ctx, gitrepo, defaultBranchCommit)
	if err != nil {
		return nil, err
	}

	workflows := make([]*api.ActionWorkflow, len(entries))
	for i, entry := range entries {
		workflows[i] = getActionWorkflowEntry(ctx, repo, gitrepo, defaultBranchCommit, git.RefNameFromBranch(repo.DefaultBranch), folder, entry)
	}

	return workflows, nil
}

func GetActionWorkflow(ctx context.Context, gitRepo *git.Repository, repo *repo_model.Repository, workflowID string) (*api.ActionWorkflow, error) {
	defaultBranchCommit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return nil, err
	}

	return getActionWorkflowFromCommit(ctx, repo, gitRepo, defaultBranchCommit, git.RefNameFromBranch(repo.DefaultBranch), workflowID)
}

func GetActionWorkflowByRef(ctx context.Context, gitrepo *git.Repository, repo *repo_model.Repository, workflowID string, ref git.RefName) (*api.ActionWorkflow, error) {
	if ref == "" {
		return nil, util.NewNotExistErrorf("workflow %q not found", workflowID)
	}

	refCommitID, err := gitrepo.GetRefCommitID(ctx, ref.String())
	if err != nil {
		return nil, err
	}
	refCommit, err := gitrepo.GetCommit(ctx, refCommitID)
	if err != nil {
		return nil, err
	}

	return getActionWorkflowFromCommit(ctx, repo, gitrepo, refCommit, ref, workflowID)
}

func getActionWorkflowFromCommit(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, commit *git.Commit, refName git.RefName, workflowID string) (*api.ActionWorkflow, error) {
	folder, entries, err := actions.ListWorkflows(ctx, gitRepo, commit)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name() == workflowID {
			return getActionWorkflowEntry(ctx, repo, gitRepo, commit, refName, folder, entry), nil
		}
	}

	return nil, util.NewNotExistErrorf("workflow %q not found", workflowID)
}

// GetScopedActionWorkflow resolves a scoped workflow definition (under SCOPED_WORKFLOW_DIRS) from the source repo at commitSHA.
func GetScopedActionWorkflow(ctx context.Context, sourceGitRepo *git.Repository, sourceRepo *repo_model.Repository, workflowID, commitSHA string) (*api.ActionWorkflow, error) {
	commit, err := sourceGitRepo.GetCommit(ctx, commitSHA)
	if err != nil {
		return nil, err
	}

	folder, entries, err := actions.ListScopedWorkflows(ctx, sourceGitRepo, commit)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.Name() == workflowID {
			// An empty ref pins HTMLURL to commit (the run's WorkflowCommitSHA) rather than the moving default branch.
			wf := getActionWorkflowEntry(ctx, sourceRepo, sourceGitRepo, commit, "", folder, entry)
			// TODO: a scoped workflow has no repo-level representation on the source: the workflow API scans WORKFLOW_DIRS (not SCOPED_WORKFLOW_DIRS),
			// and the badge only reflects the source's repo-level runs, so neither link resolves a scoped workflow.
			// Blank them for now and populate once a scoped-aware workflow/badge endpoint exists.
			wf.URL = ""
			wf.BadgeURL = ""
			return wf, nil
		}
	}

	return nil, util.NewNotExistErrorf("scoped workflow %q not found", workflowID)
}

// ResolveActionWorkflowForRun returns the api.ActionWorkflow describing a run's workflow definition.
// For a scoped run the definition lives in the source repo (run.WorkflowRepoID @ run.WorkflowCommitSHA) under SCOPED_WORKFLOW_DIRS,
// not in the consuming repo, so it is resolved against the source repo.
func ResolveActionWorkflowForRun(ctx context.Context, repo *repo_model.Repository, run *actions_model.ActionRun) (*api.ActionWorkflow, error) {
	if run.IsScopedRun {
		sourceRepo, err := repo_model.GetRepositoryByID(ctx, run.WorkflowRepoID)
		if err != nil {
			return nil, err
		}
		sourceGitRepo, err := git.OpenRepository(ctx, sourceRepo)
		if err != nil {
			return nil, err
		}
		defer sourceGitRepo.Close()
		return GetScopedActionWorkflow(ctx, sourceGitRepo, sourceRepo, run.WorkflowID, run.WorkflowCommitSHA)
	}

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	convertedWorkflow, err := GetActionWorkflowByRef(ctx, gitRepo, repo, run.WorkflowID, git.RefName(run.Ref))
	if err != nil && errors.Is(err, util.ErrNotExist) {
		convertedWorkflow, err = GetActionWorkflow(ctx, gitRepo, repo, run.WorkflowID)
	}
	return convertedWorkflow, err
}

// ToActionArtifact convert a actions_model.ActionArtifact to an api.ActionArtifact
func ToActionArtifact(repo *repo_model.Repository, art *actions_model.ActionArtifact) (*api.ActionArtifact, error) {
	url := fmt.Sprintf("%s/actions/artifacts/%d", repo.APIURL(), art.ID)

	return &api.ActionArtifact{
		ID:                 art.ID,
		Name:               art.ArtifactName,
		SizeInBytes:        art.FileSize,
		Expired:            art.Status == actions_model.ArtifactStatusExpired,
		URL:                url,
		ArchiveDownloadURL: url + "/zip",
		CreatedAt:          art.CreatedUnix.AsLocalTime(),
		UpdatedAt:          art.UpdatedUnix.AsLocalTime(),
		ExpiresAt:          art.ExpiredUnix.AsLocalTime(),
		WorkflowRun: &api.ActionWorkflowRun{
			ID:           art.RunID,
			RepositoryID: art.RepoID,
			HeadSha:      art.CommitSHA,
		},
	}, nil
}

func ToActionRunner(ctx context.Context, runner *actions_model.ActionRunner) *api.ActionRunner {
	status := runner.Status()
	apiStatus := "offline"
	if runner.IsOnline() {
		apiStatus = "online"
	}
	labels := make([]*api.ActionRunnerLabel, len(runner.AgentLabels))
	for i, label := range runner.AgentLabels {
		labels[i] = &api.ActionRunnerLabel{
			ID:   int64(i),
			Name: label,
			Type: "custom",
		}
	}
	return &api.ActionRunner{
		ID:        runner.ID,
		Name:      runner.Name,
		Status:    apiStatus,
		Busy:      status == runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE,
		Disabled:  runner.IsDisabled,
		Ephemeral: runner.Ephemeral,
		Labels:    labels,
	}
}
