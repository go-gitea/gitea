// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	webhook_module "gitea.dev/modules/webhook"
	commitstatus_service "gitea.dev/services/repository/commitstatus"
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
	// fall back to the file name when the workflow has no non-blank `name:`
	if wfs, err := jobparser.Parse(job.WorkflowPayload); err == nil && len(wfs) > 0 {
		if name := strings.TrimSpace(wfs[0].Name); name != "" {
			runName = name
		}
	}
	ctxName := strings.TrimSpace(fmt.Sprintf("%s / %s (%s)", runName, job.Name, event)) // git_model.NewCommitStatus also trims spaces
	// Mix the workflow file path into the hash so two workflow files that
	// share the same `name:` and job name produce distinct commit statuses
	// even though they render identically — matching GitHub's behavior
	// (issue #35699).
	ctxHash := git_model.HashCommitStatusContext(ctxName + "\x00" + run.WorkflowID)
	// Pre-fix rows were hashed from Context alone. If a pre-existing row with
	// the legacy hash is still the "latest" for this SHA, reuse that hash so
	// the new row supersedes it; otherwise the old pending status would stay
	// stuck forever (it lives in its own dedupe group). Only relevant for
	// in-flight workflows at upgrade time.
	legacyHash := git_model.HashCommitStatusContext(ctxName)
	state := toCommitStatus(job.Status)
	targetURL := fmt.Sprintf("%s/jobs/%d", run.Link(), job.ID)
	description := toCommitStatusDescription(job)

	statuses, err := git_model.GetLatestCommitStatus(ctx, repo.ID, commitID, db.ListOptionsAll)
	if err != nil {
		return fmt.Errorf("GetLatestCommitStatus: %w", err)
	}
	for _, v := range statuses {
		if v.ContextHash == legacyHash && v.Context == ctxName {
			ctxHash = legacyHash
			break
		}
	}
	for _, v := range statuses {
		if v.ContextHash == ctxHash {
			if v.State == state && v.TargetURL == targetURL && v.Description == description {
				return nil
			}
			break
		}
	}

	creator := user_model.NewActionsUser()
	status := git_model.CommitStatus{
		SHA:         commitID,
		TargetURL:   targetURL,
		Description: description,
		Context:     ctxName,
		ContextHash: ctxHash,
		State:       state,
		CreatorID:   creator.ID,
	}

	return commitstatus_service.CreateCommitStatus(ctx, repo, creator, commitID, &status)
}

func toCommitStatusDescription(job *actions_model.ActionRunJob) string {
	switch job.Status {
	// TODO: if we want support description in different languages, we need to support i18n placeholders in it
	case actions_model.StatusSuccess:
		return fmt.Sprintf("Successful in %s", job.Duration())
	case actions_model.StatusFailure:
		return fmt.Sprintf("Failing after %s", job.Duration())
	case actions_model.StatusCancelled:
		return fmt.Sprintf("Canceled after %s", job.Duration())
	case actions_model.StatusSkipped:
		return "Skipped"
	case actions_model.StatusRunning:
		return "In progress"
	case actions_model.StatusCancelling:
		return "Canceling"
	case actions_model.StatusWaiting:
		return "Waiting to run"
	case actions_model.StatusBlocked:
		return "Blocked by required conditions"
	default:
		return fmt.Sprintf("Unknown status: %d", job.Status)
	}
}

func toCommitStatus(status actions_model.Status) commitstatus.CommitStatusState {
	switch status {
	case actions_model.StatusSuccess:
		return commitstatus.CommitStatusSuccess
	case actions_model.StatusFailure, actions_model.StatusCancelled:
		return commitstatus.CommitStatusFailure
	case actions_model.StatusWaiting, actions_model.StatusBlocked, actions_model.StatusRunning, actions_model.StatusCancelling:
		return commitstatus.CommitStatusPending
	case actions_model.StatusSkipped:
		return commitstatus.CommitStatusSkipped
	default:
		return commitstatus.CommitStatusError
	}
}
