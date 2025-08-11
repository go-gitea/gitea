// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	actions_module "code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Cleanup removes expired actions logs, data, artifacts and used ephemeral runners
func Cleanup(ctx context.Context) error {
	// clean up expired artifacts
	if err := CleanupArtifacts(ctx); err != nil {
		return fmt.Errorf("cleanup artifacts: %w", err)
	}

	// clean up old logs
	if err := CleanupExpiredLogs(ctx); err != nil {
		return fmt.Errorf("cleanup logs: %w", err)
	}

	// clean up old ephemeral runners
	if err := CleanupEphemeralRunners(ctx); err != nil {
		return fmt.Errorf("cleanup old ephemeral runners: %w", err)
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
	log.Info("Found %d expired artifacts", len(artifacts))
	for _, artifact := range artifacts {
		if err := actions_model.SetArtifactExpired(taskCtx, artifact.ID); err != nil {
			log.Error("Cannot set artifact %d expired: %v", artifact.ID, err)
			continue
		}
		if err := storage.ActionsArtifacts.Delete(artifact.StoragePath); err != nil {
			log.Error("Cannot delete artifact %d: %v", artifact.ID, err)
			// go on
		}
		log.Info("Artifact %d is deleted (due to expiration)", artifact.ID)
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
				// go on
			}
			log.Info("Artifact %d is deleted (due to pending deletion)", artifact.ID)
		}
		if len(artifacts) < deleteArtifactBatchSize {
			log.Debug("No more artifacts pending deletion")
			break
		}
	}
	return nil
}

const deleteLogBatchSize = 100

func removeTaskLog(ctx context.Context, task *actions_model.ActionTask) {
	if err := actions_module.RemoveLogs(ctx, task.LogInStorage, task.LogFilename); err != nil {
		log.Error("Failed to remove log %s (in storage %v) of task %v: %v", task.LogFilename, task.LogInStorage, task.ID, err)
		// do not return error here, go on
	}
}

// CleanupExpiredLogs removes logs which are older than the configured retention time
func CleanupExpiredLogs(ctx context.Context) error {
	olderThan := timeutil.TimeStampNow().AddDuration(-time.Duration(setting.Actions.LogRetentionDays) * 24 * time.Hour)

	count := 0
	for {
		tasks, err := actions_model.FindOldTasksToExpire(ctx, olderThan, deleteLogBatchSize)
		if err != nil {
			return fmt.Errorf("find old tasks: %w", err)
		}
		for _, task := range tasks {
			removeTaskLog(ctx, task)
			task.LogIndexes = nil // clear log indexes since it's a heavy field
			task.LogExpired = true
			if err := actions_model.UpdateTask(ctx, task, "log_indexes", "log_expired"); err != nil {
				log.Error("Failed to update task %v: %v", task.ID, err)
				// do not return error here, continue to next task
				continue
			}
			count++
			log.Trace("Removed log %s of task %v", task.LogFilename, task.ID)
		}
		if len(tasks) < deleteLogBatchSize {
			break
		}
	}

	log.Info("Removed %d logs", count)
	return nil
}

// CleanupEphemeralRunners removes used ephemeral runners which are no longer able to process jobs
func CleanupEphemeralRunners(ctx context.Context) error {
	subQuery := builder.Select("`action_runner`.id").
		From(builder.Select("*").From("`action_runner`"), "`action_runner`"). // mysql needs this redundant subquery
		Join("INNER", "`action_task`", "`action_task`.`runner_id` = `action_runner`.`id`").
		Where(builder.Eq{"`action_runner`.`ephemeral`": true}).
		And(builder.NotIn("`action_task`.`status`", actions_model.StatusWaiting, actions_model.StatusRunning, actions_model.StatusBlocked))
	b := builder.Delete(builder.In("id", subQuery)).From("`action_runner`")
	res, err := db.GetEngine(ctx).Exec(b)
	if err != nil {
		return fmt.Errorf("find runners: %w", err)
	}
	affected, _ := res.RowsAffected()
	log.Info("Removed %d runners", affected)
	return nil
}

// CleanupEphemeralRunnersByPickedTaskOfRepo removes all ephemeral runners that have active/finished tasks on the given repository
func CleanupEphemeralRunnersByPickedTaskOfRepo(ctx context.Context, repoID int64) error {
	subQuery := builder.Select("`action_runner`.id").
		From(builder.Select("*").From("`action_runner`"), "`action_runner`"). // mysql needs this redundant subquery
		Join("INNER", "`action_task`", "`action_task`.`runner_id` = `action_runner`.`id`").
		Where(builder.And(builder.Eq{"`action_runner`.`ephemeral`": true}, builder.Eq{"`action_task`.`repo_id`": repoID}))
	b := builder.Delete(builder.In("id", subQuery)).From("`action_runner`")
	res, err := db.GetEngine(ctx).Exec(b)
	if err != nil {
		return fmt.Errorf("find runners: %w", err)
	}
	affected, _ := res.RowsAffected()
	log.Info("Removed %d runners", affected)
	return nil
}

// DeleteRun deletes workflow run, including all logs and artifacts.
func DeleteRun(ctx context.Context, run *actions_model.ActionRun) error {
	if !run.Status.IsDone() {
		return errors.New("run is not done")
	}

	repoID := run.RepoID

	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		return err
	}
	jobIDs := container.FilterSlice(jobs, func(j *actions_model.ActionRunJob) (int64, bool) {
		return j.ID, true
	})
	tasks := make(actions_model.TaskList, 0)
	if len(jobIDs) > 0 {
		if err := db.GetEngine(ctx).Where("repo_id = ?", repoID).In("job_id", jobIDs).Find(&tasks); err != nil {
			return err
		}
	}

	artifacts, err := db.Find[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RepoID: repoID,
		RunID:  run.ID,
	})
	if err != nil {
		return err
	}

	var recordsToDelete []any

	recordsToDelete = append(recordsToDelete, &actions_model.ActionRun{
		RepoID: repoID,
		ID:     run.ID,
	})
	recordsToDelete = append(recordsToDelete, &actions_model.ActionRunJob{
		RepoID: repoID,
		RunID:  run.ID,
	})
	for _, tas := range tasks {
		recordsToDelete = append(recordsToDelete, &actions_model.ActionTask{
			RepoID: repoID,
			ID:     tas.ID,
		})
		recordsToDelete = append(recordsToDelete, &actions_model.ActionTaskStep{
			RepoID: repoID,
			TaskID: tas.ID,
		})
		recordsToDelete = append(recordsToDelete, &actions_model.ActionTaskOutput{
			TaskID: tas.ID,
		})
	}
	recordsToDelete = append(recordsToDelete, &actions_model.ActionArtifact{
		RepoID: repoID,
		RunID:  run.ID,
	})

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// TODO: Deleting task records could break current ephemeral runner implementation. This is a temporary workaround suggested by ChristopherHX.
		// Since you delete potentially the only task an ephemeral act_runner has ever run, please delete the affected runners first.
		// one of
		//    call cleanup ephemeral runners first
		//    delete affected ephemeral act_runners
		//    I would make ephemeral runners fully delete directly before formally finishing the task
		//
		// See also: https://github.com/go-gitea/gitea/pull/34337#issuecomment-2862222788
		if err := CleanupEphemeralRunners(ctx); err != nil {
			return err
		}
		return db.DeleteBeans(ctx, recordsToDelete...)
	}); err != nil {
		return err
	}

	// Delete files on storage
	for _, tas := range tasks {
		removeTaskLog(ctx, tas)
	}
	for _, art := range artifacts {
		if err := storage.ActionsArtifacts.Delete(art.StoragePath); err != nil {
			log.Error("remove artifact file %q: %v", art.StoragePath, err)
		}
	}

	return nil
}
