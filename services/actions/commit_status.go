// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"path"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/nektos/act/pkg/jobparser"
)

// CreateCommitStatus creates a commit status for the given job.
// It won't return an error failed, but will log it, because it's not critical.
func CreateCommitStatus(ctx context.Context, jobs ...*actions_model.ActionRunJob) {
	for _, job := range jobs {
		if err := createCommitStatus(ctx, job); err != nil {
			log.Error("Failed to create commit status for job %d: %v", job.ID, err)
		}
	}
}

func createCommitStatus(ctx context.Context, job *actions_model.ActionRunJob) error {
	if err := job.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	run := job.Run

	var (
		sha   string
		event string
	)
	switch run.Event {
	case webhook_module.HookEventPush:
		event = "push"
		payload, err := run.GetPushEventPayload()
		if err != nil {
			return fmt.Errorf("GetPushEventPayload: %w", err)
		}
		if payload.HeadCommit == nil {
			return fmt.Errorf("head commit is missing in event payload")
		}
		sha = payload.HeadCommit.ID
	case webhook_module.HookEventPullRequest, webhook_module.HookEventPullRequestSync:
		event = "pull_request"
		payload, err := run.GetPullRequestEventPayload()
		if err != nil {
			return fmt.Errorf("GetPullRequestEventPayload: %w", err)
		}
		if payload.PullRequest == nil {
			return fmt.Errorf("pull request is missing in event payload")
		} else if payload.PullRequest.Head == nil {
			return fmt.Errorf("head of pull request is missing in event payload")
		}
		sha = payload.PullRequest.Head.Sha
	default:
		return nil
	}

	repo := run.Repo
	// TODO: store workflow name as a field in ActionRun to avoid parsing
	runName := path.Base(run.WorkflowID)
	if wfs, err := jobparser.Parse(job.WorkflowPayload); err == nil && len(wfs) > 0 {
		runName = wfs[0].Name
	}
	ctxname := fmt.Sprintf("%s / %s (%s)", runName, job.Name, event)
	state := toCommitStatus(job.Status)
	if statuses, _, err := git_model.GetLatestCommitStatus(ctx, repo.ID, sha, db.ListOptions{ListAll: true}); err == nil {
		for _, v := range statuses {
			if v.Context == ctxname {
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

	description := ""
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
	}

	index, err := getIndexOfJob(ctx, job)
	if err != nil {
		return fmt.Errorf("getIndexOfJob: %w", err)
	}

	creator := user_model.NewActionsUser()
	if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
		Repo:    repo,
		SHA:     sha,
		Creator: creator,
		CommitStatus: &git_model.CommitStatus{
			SHA:         sha,
			TargetURL:   fmt.Sprintf("%s/jobs/%d", run.Link(), index),
			Description: description,
			Context:     ctxname,
			CreatorID:   creator.ID,
			State:       state,
		},
	}); err != nil {
		return fmt.Errorf("NewCommitStatus: %w", err)
	}

	return nil
}

func toCommitStatus(status actions_model.Status) api.CommitStatusState {
	switch status {
	case actions_model.StatusSuccess, actions_model.StatusSkipped:
		return api.CommitStatusSuccess
	case actions_model.StatusFailure:
		return api.CommitStatusFailure
	case actions_model.StatusCancelled:
		return api.CommitStatusWarning
	case actions_model.StatusWaiting, actions_model.StatusBlocked:
		return api.CommitStatusPending
	case actions_model.StatusRunning:
		return api.CommitStatusRunning
	default:
		return api.CommitStatusError
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
