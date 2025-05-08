// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// DeleteRun deletes a done workflow run
func DeleteRun(ctx context.Context, repoID int64, run *actions.ActionRun, jobs []*actions.ActionRunJob) error {
	tasks := make(actions.TaskList, 0)

	jobIDs := container.FilterSlice(jobs, func(j *actions.ActionRunJob) (int64, bool) {
		return j.ID, j.ID != 0
	})
	if len(jobIDs) > 0 {
		if err := db.GetEngine(ctx).Where("repo_id = ?", repoID).In("job_id", jobIDs).Find(&tasks); err != nil {
			return err
		}
	}

	artifacts, err := db.Find[actions.ActionArtifact](ctx, actions.FindArtifactsOptions{
		RepoID: repoID,
		RunID:  run.ID,
	})
	if err != nil {
		return err
	}

	var recordsToDelete []any

	for _, tas := range tasks {
		recordsToDelete = append(recordsToDelete, &actions.ActionTask{
			RepoID: repoID,
			ID:     tas.ID,
		})
		recordsToDelete = append(recordsToDelete, &actions.ActionTaskStep{
			RepoID: repoID,
			TaskID: tas.ID,
		})
		recordsToDelete = append(recordsToDelete, &actions.ActionTaskOutput{
			TaskID: tas.ID,
		})
	}
	recordsToDelete = append(recordsToDelete, &actions.ActionRunJob{
		RepoID: repoID,
		RunID:  run.ID,
	})
	recordsToDelete = append(recordsToDelete, &actions.ActionRun{
		RepoID: repoID,
		ID:     run.ID,
	})
	recordsToDelete = append(recordsToDelete, &actions.ActionArtifact{
		RepoID: repoID,
		RunID:  run.ID,
	})

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		return db.DeleteBeans(ctx, recordsToDelete...)
	}); err != nil {
		return err
	}

	// Delete files on storage
	for _, tas := range tasks {
		if err := RemoveLogs(ctx, tas.LogInStorage, tas.LogFilename); err != nil {
			log.Error("remove log file %q: %v", tas.LogFilename, err)
		}
	}
	for _, art := range artifacts {
		if err := storage.ActionsArtifacts.Delete(art.StoragePath); err != nil {
			log.Error("remove artifact file %q: %v", art.StoragePath, err)
		}
	}

	return nil
}
