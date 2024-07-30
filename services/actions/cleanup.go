// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
)

// Cleanup removes expired actions logs, data and artifacts
func Cleanup(ctx context.Context) error {
	// clean up expired artifacts
	if err := CleanupArtifacts(ctx); err != nil {
		return fmt.Errorf("cleanup artifacts: %w", err)
	}

	// clean up old logs
	if err := CleanupLogs(ctx); err != nil {
		return fmt.Errorf("cleanup logs: %w", err)
	}

	return nil
}

// CleanupArtifacts removes expired add need-deleted artifacts and set records expired status
func CleanupArtifacts(taskCtx context.Context) error {
	if err := cleanExpiredArtifacts(taskCtx); err != nil {
		return err
	}
	return cleanNeedDeleteArtifacts(taskCtx)
}

func cleanExpiredArtifacts(taskCtx context.Context) error {
	artifacts, err := actions_model.ListNeedExpiredArtifacts(taskCtx)
	if err != nil {
		return err
	}
	log.Info("Found ∂%d expired artifacts", len(artifacts))
	for _, artifact := range artifacts {
		if err := actions_model.SetArtifactExpired(taskCtx, artifact.ID); err != nil {
			log.Error("Cannot set artifact %d expired: %v", artifact.ID, err)
			continue
		}
		if err := storage.ActionsArtifacts.Delete(artifact.StoragePath); err != nil {
			log.Error("Cannot delete artifact %d: %v", artifact.ID, err)
			continue
		}
		log.Info("Artifact %d set expired", artifact.ID)
	}
	return nil
}

// deleteArtifactBatchSize is the batch size of deleting artifacts
const deleteArtifactBatchSize = 100

func cleanNeedDeleteArtifacts(taskCtx context.Context) error {
	for {
		artifacts, err := actions_model.ListPendingDeleteArtifacts(taskCtx, deleteArtifactBatchSize)
		if err != nil {
			return err
		}
		log.Info("Found %d artifacts pending deletion", len(artifacts))
		for _, artifact := range artifacts {
			if err := actions_model.SetArtifactDeleted(taskCtx, artifact.ID); err != nil {
				log.Error("Cannot set artifact %d deleted: %v", artifact.ID, err)
				continue
			}
			if err := storage.ActionsArtifacts.Delete(artifact.StoragePath); err != nil {
				log.Error("Cannot delete artifact %d: %v", artifact.ID, err)
				continue
			}
			log.Info("Artifact %d set deleted", artifact.ID)
		}
		if len(artifacts) < deleteArtifactBatchSize {
			log.Debug("No more artifacts pending deletion")
			break
		}
	}
	return nil
}

// CleanupLogs removes logs which are older than the configured retention time
func CleanupLogs(ctx context.Context) error {
	before := timeutil.TimeStampNow().AddDuration(-time.Duration(setting.Actions.LogRetentionDays) * 24 * time.Hour)

	count := 0
	if err := actions_model.IterateOldTasks(ctx, before, func(ctx context.Context, task *actions_model.ActionTask) error {
		err := actions_module.RemoveLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			log.Error("Failed to remove log %s (in storage %v) of task %v: %v", task.LogFilename, task.LogInStorage, task.ID, err)
			// do not return error here, continue to next task
			return nil
		}

		task.LogIndexes = nil // clear log indexes since it's a heavy field
		task.LogExpired = true
		if err := actions_model.UpdateTask(ctx, task, "log_indexes", "log_expired"); err != nil {
			log.Warn("Failed to update task %v: %v", task.ID, err)
			// do not return error here, continue to next task
			return nil
		}

		log.Trace("Removed log %s of task %v", task.LogFilename, task.ID)
		count++

		return nil
	}); err != nil {
		if errors.Is(err, ctx.Err()) {
			return nil
		}
		return fmt.Errorf("iterate old tasks: %w", err)
	}

	log.Info("Removed %d logs", count)
	return nil
}
