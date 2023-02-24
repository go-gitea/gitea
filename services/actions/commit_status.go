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
	if run.Event != webhook_module.HookEventPush {
		return nil
	}

	payload, err := run.GetPushEventPayload()
	if err != nil {
		return fmt.Errorf("GetPushEventPayload: %w", err)
	}

	creator, err := user_model.GetUserByID(ctx, payload.Pusher.ID)
	if err != nil {
		return fmt.Errorf("GetUserByID: %w", err)
	}

	repo := run.Repo
	sha := payload.HeadCommit.ID
	ctxname := job.Name
	state := toCommitStatus(job.Status)

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
		SHA:     payload.HeadCommit.ID,
		Creator: creator,
		CommitStatus: &git_model.CommitStatus{
			SHA:         sha,
			TargetURL:   run.Link(),
			Description: "",
			Context:     ctxname,
			CreatorID:   payload.Pusher.ID,
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
