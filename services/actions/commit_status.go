// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	gitea_context "code.gitea.io/gitea/services/context"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
)

// CreateCommitStatusForRunJobs creates a commit status for the given job if it has a supported event and related commit.
// It won't return an error failed, but will log it, because it's not critical.
func CreateCommitStatusForRunJobs(ctx context.Context, run *actions_model.ActionRun, jobs ...*actions_model.ActionRunJob) {
	// don't create commit status for cron job
	if run.ScheduleID != 0 {
		return
	}

	event, commitID, err := getCommitStatusEventNameAndCommitID(run)
	if err != nil {
		log.Error("GetCommitStatusEventNameAndSHA: %v", err)
	}
	if event == "" || commitID == "" {
		return // unsupported event, or no commit id, or error occurs, do nothing
	}

	if err = run.LoadAttributes(ctx); err != nil {
		log.Error("run.LoadAttributes: %v", err)
		return
	}

	for _, job := range jobs {
		if err = createCommitStatus(ctx, run.Repo, event, commitID, run, job); err != nil {
			log.Error("Failed to create commit status for job %d: %v", job.ID, err)
		}
	}
}

func GetRunsFromCommitStatuses(ctx context.Context, statuses []*git_model.CommitStatus) ([]*actions_model.ActionRun, error) {
	runMap := make(map[int64]*actions_model.ActionRun)
	for _, status := range statuses {
		runID, _, ok := status.ParseGiteaActionsTargetURL(ctx)
		if !ok {
			continue
		}
		_, ok = runMap[runID]
		if !ok {
			run, err := actions_model.GetRunByRepoAndID(ctx, status.RepoID, runID)
			if err != nil {
				if errors.Is(err, util.ErrNotExist) {
					// the run may be deleted manually, just skip it
					continue
				}
				return nil, fmt.Errorf("GetRunByRepoAndID: %w", err)
			}
			runMap[runID] = run
		}
	}
	runs := make([]*actions_model.ActionRun, 0, len(runMap))
	for _, run := range runMap {
		runs = append(runs, run)
	}
	return runs, nil
}

// CommitStatusActionInfo maps CommitStatus.ID to the live ActionRunJob status
// for pending Gitea Actions rows. Kept off the CommitStatus model since rows
// can also come from external CIs or the API and must keep their stored meaning.
type CommitStatusActionInfo map[int64]actions_model.Status

// IconStatus returns the action status name to render the icon for the given
// CommitStatus, or "" when the regular commit_status icon should be used.
func (m CommitStatusActionInfo) IconStatus(s *git_model.CommitStatus) string {
	if status, ok := m[s.ID]; ok {
		return status.String()
	}
	return ""
}

// Description returns the live waiting/blocked/running text or s.Description.
// createCommitStatus dedup is keyed only on State, so a Pending row's stored
// Description freezes at whichever of the three was written first.
func (m CommitStatusActionInfo) Description(s *git_model.CommitStatus) string {
	switch m[s.ID] {
	case actions_model.StatusWaiting:
		return "Waiting to run"
	case actions_model.StatusRunning:
		return "In progress"
	case actions_model.StatusBlocked:
		return "Blocked by required conditions"
	}
	return s.Description
}

// PrepareCommitStatusesUI hides Gitea Actions URLs for users without Actions
// read permission and stores the action info enrichment under
// ctx.Data["CommitStatusActionInfo"] for templates like repo/pulls/status.tmpl.
func PrepareCommitStatusesUI(ctx *gitea_context.Context, statuses []*git_model.CommitStatus) {
	if !ctx.Repo.CanRead(unit.TypeActions) {
		git_model.CommitStatusesHideActionsURL(ctx, statuses)
	}
	ctx.Data["CommitStatusActionInfo"] = GetCommitStatusActionInfo(ctx, statuses)
}

// PrepareCommitStatusesMapUI is the map variant of PrepareCommitStatusesUI for
// callers that hold statuses keyed by commit-or-issue ID.
func PrepareCommitStatusesMapUI[K comparable](ctx *gitea_context.Context, m map[K][]*git_model.CommitStatus) {
	var flat []*git_model.CommitStatus
	for _, cs := range m {
		flat = append(flat, cs...)
	}
	PrepareCommitStatusesUI(ctx, flat)
}

// GetCommitStatusActionInfo resolves the live ActionRunJob.Status for any
// pending CommitStatus rows backed by Gitea Actions. Non-pending statuses are
// not enriched: they're trusted as-is regardless of source.
func GetCommitStatusActionInfo(ctx context.Context, statuses []*git_model.CommitStatus) CommitStatusActionInfo {
	if len(statuses) == 0 {
		return nil
	}
	statusByJobID := make(map[int64]*git_model.CommitStatus)
	// Cache repo per RepoID across ParseGiteaActionsTargetURL lazy-loads,
	// same as CommitStatusesHideActionsURL.
	repoCache := make(map[int64]*repo_model.Repository)
	for _, status := range statuses {
		if status == nil || status.State != commitstatus.CommitStatusPending {
			continue
		}
		if status.Repo == nil {
			status.Repo = repoCache[status.RepoID]
		}
		_, jobID, ok := status.ParseGiteaActionsTargetURL(ctx)
		repoCache[status.RepoID] = status.Repo
		if ok {
			statusByJobID[jobID] = status
		}
	}
	if len(statusByJobID) == 0 {
		return nil
	}
	jobIDs := make([]int64, 0, len(statusByJobID))
	for id := range statusByJobID {
		jobIDs = append(jobIDs, id)
	}
	jobs := make(map[int64]*actions_model.ActionRunJob, len(jobIDs))
	if err := db.GetEngine(ctx).In("id", jobIDs).Cols("id", "status").Find(&jobs); err != nil {
		log.Error("GetCommitStatusActionInfo: find action run jobs: %v", err)
		return nil
	}
	info := make(CommitStatusActionInfo, len(jobs))
	for jobID, status := range statusByJobID {
		if job, ok := jobs[jobID]; ok && !job.Status.IsUnknown() {
			info[status.ID] = job.Status
		}
	}
	return info
}

func getCommitStatusEventNameAndCommitID(run *actions_model.ActionRun) (event, commitID string, _ error) {
	switch run.Event {
	case webhook_module.HookEventPush:
		event = "push"
		payload, err := run.GetPushEventPayload()
		if err != nil {
			return "", "", fmt.Errorf("GetPushEventPayload: %w", err)
		}
		if payload.HeadCommit == nil {
			return "", "", errors.New("head commit is missing in event payload")
		}
		commitID = payload.HeadCommit.ID
	case // pull_request
		webhook_module.HookEventPullRequest,
		webhook_module.HookEventPullRequestSync,
		webhook_module.HookEventPullRequestAssign,
		webhook_module.HookEventPullRequestLabel,
		webhook_module.HookEventPullRequestReviewRequest,
		webhook_module.HookEventPullRequestMilestone:
		if run.TriggerEvent == actions_module.GithubEventPullRequestTarget {
			event = "pull_request_target"
		} else {
			event = "pull_request"
		}
		payload, err := run.GetPullRequestEventPayload()
		if err != nil {
			return "", "", fmt.Errorf("GetPullRequestEventPayload: %w", err)
		}
		if payload.PullRequest == nil {
			return "", "", errors.New("pull request is missing in event payload")
		} else if payload.PullRequest.Head == nil {
			return "", "", errors.New("head of pull request is missing in event payload")
		}
		commitID = payload.PullRequest.Head.Sha
	case // pull_request_review events share the same PullRequestPayload as pull_request
		webhook_module.HookEventPullRequestReviewApproved,
		webhook_module.HookEventPullRequestReviewRejected,
		webhook_module.HookEventPullRequestReviewComment:
		event = run.TriggerEvent
		payload, err := run.GetPullRequestEventPayload()
		if err != nil {
			return "", "", fmt.Errorf("GetPullRequestEventPayload: %w", err)
		}
		if payload.PullRequest == nil {
			return "", "", errors.New("pull request is missing in event payload")
		} else if payload.PullRequest.Head == nil {
			return "", "", errors.New("head of pull request is missing in event payload")
		}
		commitID = payload.PullRequest.Head.Sha
	case webhook_module.HookEventRelease:
		event = string(run.Event)
		commitID = run.CommitSHA
	default: // do nothing, return empty
	}
	return event, commitID, nil
}

func createCommitStatus(ctx context.Context, repo *repo_model.Repository, event, commitID string, run *actions_model.ActionRun, job *actions_model.ActionRunJob) error {
	// TODO: store workflow name as a field in ActionRun to avoid parsing
	runName := path.Base(run.WorkflowID)
	if wfs, err := jobparser.Parse(job.WorkflowPayload); err == nil && len(wfs) > 0 {
		runName = wfs[0].Name
	}
	ctxName := fmt.Sprintf("%s / %s (%s)", runName, job.Name, event)
	ctxName = strings.TrimSpace(ctxName) // git_model.NewCommitStatus also trims spaces
	state := toCommitStatus(job.Status)
	if statuses, err := git_model.GetLatestCommitStatus(ctx, repo.ID, commitID, db.ListOptionsAll); err == nil {
		for _, v := range statuses {
			if v.Context == ctxName {
				if v.State == state {
					// no need to update
					return nil
				}
				break
			}
		}
	} else {
		return fmt.Errorf("GetLatestCommitStatus: %w", err)
	}

	var description string
	switch job.Status {
	// TODO: if we want support description in different languages, we need to support i18n placeholders in it
	case actions_model.StatusSuccess:
		description = fmt.Sprintf("Successful in %s", job.Duration())
	case actions_model.StatusFailure:
		description = fmt.Sprintf("Failing after %s", job.Duration())
	case actions_model.StatusCancelled:
		description = "Has been cancelled"
	case actions_model.StatusSkipped:
		description = "Has been skipped"
	case actions_model.StatusRunning:
		description = "In progress"
	case actions_model.StatusWaiting:
		description = "Waiting to run"
	case actions_model.StatusBlocked:
		description = "Blocked by required conditions"
	default:
		description = "Unknown status: " + strconv.Itoa(int(job.Status))
	}

	creator := user_model.NewActionsUser()
	status := git_model.CommitStatus{
		SHA:         commitID,
		TargetURL:   fmt.Sprintf("%s/jobs/%d", run.Link(), job.ID),
		Description: description,
		Context:     ctxName,
		CreatorID:   creator.ID,
		State:       state,
	}

	return commitstatus_service.CreateCommitStatus(ctx, repo, creator, commitID, &status)
}

func toCommitStatus(status actions_model.Status) commitstatus.CommitStatusState {
	switch status {
	case actions_model.StatusSuccess:
		return commitstatus.CommitStatusSuccess
	case actions_model.StatusFailure, actions_model.StatusCancelled:
		return commitstatus.CommitStatusFailure
	case actions_model.StatusWaiting, actions_model.StatusBlocked, actions_model.StatusRunning:
		return commitstatus.CommitStatusPending
	case actions_model.StatusSkipped:
		return commitstatus.CommitStatusSkipped
	default:
		return commitstatus.CommitStatusError
	}
}
