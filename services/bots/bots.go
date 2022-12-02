// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"context"
	"fmt"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	bots_module "code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
)

func Init() {
	jobEmitterQueue = queue.CreateUniqueQueue("bots_ready_job", jobEmitterQueueHandle, new(jobUpdate))
	go graceful.GetManager().RunWithShutdownFns(jobEmitterQueue.Run)
}

func DeleteResourceOfRepository(ctx context.Context, repo *repo_model.Repository) error {
	tasks, _, err := bots_model.FindTasks(ctx, bots_model.FindTaskOptions{RepoID: repo.ID})
	if err != nil {
		return fmt.Errorf("find task of repo %v: %w", repo.ID, err)
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)
		if _, err := e.Delete(&bots_model.BotTaskStep{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("delete bots task steps of repo %d: %w", repo.ID, err)
		}
		if _, err := e.Delete(&bots_model.BotTask{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("delete bots tasks of repo %d: %w", repo.ID, err)
		}
		if _, err := e.Delete(&bots_model.BotRunJob{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("delete bots run jobs of repo %d: %w", repo.ID, err)
		}
		if _, err := e.Delete(&bots_model.BotRun{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("delete bots runs of repo %d: %w", repo.ID, err)
		}
		if _, err := e.Delete(&bots_model.BotRunner{RepoID: repo.ID}); err != nil {
			return fmt.Errorf("delete bots runner of repo %d: %w", repo.ID, err)
		}
		return nil
	}); err != nil {
		return err
	}

	// remove logs file after tasks have been deleted, to avoid new log files
	for _, task := range tasks {
		err := bots_module.RemoveLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			log.Error("remove log file %q: %v", task.LogFilename, err)
			// go on
		}
	}

	return nil
}

func CreateCommitStatus(ctx context.Context, job *bots_model.BotRunJob) error {
	if err := job.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	run := job.Run
	if run.Event != webhook.HookEventPush {
		return nil
	}

	payload, err := run.GetPushEventPayload()
	if err != nil {
		return fmt.Errorf("GetPushEventPayload: %w", err)
	}

	creator, err := user_model.GetUserByID(payload.Pusher.ID)
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

	if err := git_model.NewCommitStatus(git_model.NewCommitStatusOptions{
		Repo:    repo,
		SHA:     payload.HeadCommit.ID,
		Creator: creator,
		CommitStatus: &git_model.CommitStatus{
			SHA:         sha,
			TargetURL:   run.HTMLURL(),
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

func toCommitStatus(status bots_model.Status) api.CommitStatusState {
	switch status {
	case bots_model.StatusSuccess:
		return api.CommitStatusSuccess
	case bots_model.StatusFailure, bots_model.StatusCancelled, bots_model.StatusSkipped:
		return api.CommitStatusFailure
	case bots_model.StatusWaiting, bots_model.StatusBlocked:
		return api.CommitStatusPending
	case bots_model.StatusRunning:
		return api.CommitStatusRunning
	default:
		return api.CommitStatusError
	}
}
