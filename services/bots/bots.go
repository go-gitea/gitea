// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"fmt"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	bots_module "code.gitea.io/gitea/modules/bots"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
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
