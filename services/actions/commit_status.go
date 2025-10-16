// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/log"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"

	"github.com/nektos/act/pkg/jobparser"
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

func GetRunsAndJobsFromCommitStatuses(ctx context.Context, statuses []*git_model.CommitStatus) ([]*actions_model.ActionRun, []*actions_model.ActionRunJob, error) {
	jobMap := make(map[int64]*actions_model.ActionRunJob)
	runMap := make(map[int64]*actions_model.ActionRun)
	jobsMap := make(map[int64][]*actions_model.ActionRunJob)
	for _, status := range statuses {
		if !status.CreatedByGiteaActions(ctx) {
			continue
		}
		runIndex, jobIndex, err := getActionRunAndJobIndexFromCommitStatus(status)
		if err != nil {
			return nil, nil, fmt.Errorf("getActionRunAndJobIndexFromCommitStatus: %w", err)
		}
		run, ok := runMap[runIndex]
		if !ok {
			run, err = actions_model.GetRunByIndex(ctx, status.RepoID, runIndex)
			if err != nil {
				return nil, nil, fmt.Errorf("GetRunByIndex: %w", err)
			}
			runMap[runIndex] = run
		}
		jobs, ok := jobsMap[runIndex]
		if !ok {
			jobs, err = actions_model.GetRunJobsByRunID(ctx, run.ID)
			if err != nil {
				return nil, nil, fmt.Errorf("GetRunJobByIndex: %w", err)
			}
			jobsMap[runIndex] = jobs
		}
		if jobIndex < 0 || jobIndex >= int64(len(jobs)) {
			return nil, nil, fmt.Errorf("job index %d out of range for run %d", jobIndex, runIndex)
		}
		job := jobs[jobIndex]
		jobMap[job.ID] = job
	}
	runs := make([]*actions_model.ActionRun, 0, len(runMap))
	for _, run := range runMap {
		runs = append(runs, run)
	}
	jobs := make([]*actions_model.ActionRunJob, 0, len(jobMap))
	for _, job := range jobMap {
		jobs = append(jobs, job)
	}
	return runs, jobs, nil
}

func getActionRunAndJobIndexFromCommitStatus(status *git_model.CommitStatus) (int64, int64, error) {
	actionsLink, _ := strings.CutPrefix(status.TargetURL, status.Repo.Link()+"/actions/")
	// actionsLink should be like "runs/<run_index>/jobs/<job_index>"

	re := regexp.MustCompile(`runs/(\d+)/jobs/(\d+)`)
	matches := re.FindStringSubmatch(actionsLink)

	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("%s is not a Gitea Actions link", status.TargetURL)
	}

	runIndex, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse run index: %w", err)
	}
	jobIndex, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse job index: %w", err)
	}

	return runIndex, jobIndex, nil
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
		description = "Has started running"
	case actions_model.StatusWaiting:
		description = "Waiting to run"
	case actions_model.StatusBlocked:
		description = "Blocked by required conditions"
	default:
		description = "Unknown status: " + strconv.Itoa(int(job.Status))
	}

	index, err := getIndexOfJob(ctx, job)
	if err != nil {
		return fmt.Errorf("getIndexOfJob: %w", err)
	}

	creator := user_model.NewActionsUser()
	status := git_model.CommitStatus{
		SHA:         commitID,
		TargetURL:   fmt.Sprintf("%s/jobs/%d", run.Link(), index),
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

func getIndexOfJob(ctx context.Context, job *actions_model.ActionRunJob) (int, error) {
	// TODO: store job index as a field in ActionRunJob to avoid this
	jobs, err := actions_model.GetRunJobsByRunID(ctx, job.RunID)
	if err != nil {
		return 0, err
	}
	for i, v := range jobs {
		if v.ID == job.ID {
			return i, nil
		}
	}
	return 0, nil
}
