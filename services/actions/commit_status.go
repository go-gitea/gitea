// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

func CreateCommitStatus(ctx context.Context, job *actions_model.ActionRunJob) error {
	if err := job.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	run := job.Run
	var (
		sha       string
		creatorID int64
	)

	switch run.Event {
	case webhook_module.HookEventPush:
		payload, err := run.GetPushEventPayload()
		if err != nil {
			return fmt.Errorf("GetPushEventPayload: %w", err)
		}

		// Since the payload comes from json data, we should check if it's broken, or it will cause panic
		switch {
		case payload.Repo == nil:
			return fmt.Errorf("repo is missing in event payload")
		case payload.Pusher == nil:
			return fmt.Errorf("pusher is missing in event payload")
		case payload.HeadCommit == nil:
			return fmt.Errorf("head commit is missing in event payload")
		}

		sha = payload.HeadCommit.ID
		creatorID = payload.Pusher.ID
	case webhook_module.HookEventPullRequest, webhook_module.HookEventPullRequestSync:
		payload, err := run.GetPullRequestEventPayload()
		if err != nil {
			return fmt.Errorf("GetPullRequestEventPayload: %w", err)
		}

		switch {
		case payload.PullRequest == nil:
			return fmt.Errorf("pull request is missing in event payload")
		case payload.PullRequest.Head == nil:
			return fmt.Errorf("head of pull request is missing in event payload")
		case payload.PullRequest.Head.Repository == nil:
			return fmt.Errorf("head repository of pull request is missing in event payload")
		case payload.PullRequest.Head.Repository.Owner == nil:
			return fmt.Errorf("owner of head repository of pull request is missing in evnt payload")
		}

		sha = payload.PullRequest.Head.Sha
		creatorID = payload.PullRequest.Head.Repository.Owner.ID
	default:
		return nil
	}

	repo := run.Repo
	ctxname := job.Name
	state := toCommitStatus(job.Status)
	creator, err := user_model.GetUserByID(ctx, creatorID)
	if err != nil {
		return fmt.Errorf("GetUserByID: %w", err)
	}
	if statuses, _, err := git_model.GetLatestCommitStatus(ctx, repo.ID, sha, db.ListOptions{}); err == nil {
		for _, v := range statuses {
			if v.Context == ctxname {
				if v.State == state {
					return nil
				}
				break
			}
		}
	} else {
		return fmt.Errorf("GetLatestCommitStatus: %w", err)
	}

	if err := git_model.NewCommitStatus(ctx, git_model.NewCommitStatusOptions{
		Repo:    repo,
		SHA:     sha,
		Creator: creator,
		CommitStatus: &git_model.CommitStatus{
			SHA:         sha,
			TargetURL:   run.Link(),
			Description: "",
			Context:     ctxname,
			CreatorID:   creatorID,
			State:       state,
		},
	}); err != nil {
		return fmt.Errorf("NewCommitStatus: %w", err)
	}

	return nil
}

func toCommitStatus(status actions_model.Status) api.CommitStatusState {
	switch status {
	case actions_model.StatusSuccess:
		return api.CommitStatusSuccess
	case actions_model.StatusFailure, actions_model.StatusCancelled, actions_model.StatusSkipped:
		return api.CommitStatusFailure
	case actions_model.StatusWaiting, actions_model.StatusBlocked:
		return api.CommitStatusPending
	case actions_model.StatusRunning:
		return api.CommitStatusRunning
	default:
		return api.CommitStatusError
	}
}
