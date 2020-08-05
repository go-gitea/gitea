// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	
	"xorm.io/builder"
)

// PruneHookTaskTable deletes rows from hook_task as needed.
func PruneHookTaskTable(ctx context.Context) error {
	log.Error("Doing: PruneHookTaskTable")

	if err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Expr("id>0 AND is_hook_task_purge_enabled=?", true),
		func(idx int, bean interface{}) error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("Aborted due to shutdown")
			default:
			}
			repo := bean.(*models.Repository)
			repoPath := repo.RepoPath()
			log.Trace("Running prune hook_task table on repository %s", repoPath)
			if err := models.DeleteDeliveredHookTasks(repo.ID, repo.NumberWebhookDeliveriesToKeep); err != nil {
				desc := fmt.Sprintf("Failed to prune hook_task on repository (%s): %v", repoPath, err)
				log.Warn(desc)
				if err = models.CreateRepositoryNotice(desc); err != nil {
					log.Error("CreateRepositoryNotice: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Error("Finished: PruneHookTaskTable")
	return nil
}
