// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/log"
	webhook_module "code.gitea.io/gitea/modules/webhook"
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

	return git_model.UpdatCheckRunForAction(ctx, job, event, sha)
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
