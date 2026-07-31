// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/container"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	context_module "gitea.dev/services/context"
)

// logFileNameReplacer strips path traversal and header-breaking characters from user-controlled names.
var logFileNameReplacer = strings.NewReplacer(`"`, "", "\r", "", "\n", "", "/", "-", `\`, "-", "..", "__")

func sanitizeWorkflowFileName(workflowID string) string {
	if p := strings.Index(workflowID, "."); p > 0 {
		workflowID = workflowID[:p]
	}
	return logFileNameReplacer.Replace(workflowID)
}

func jobLogFileName(workflowID, jobName string, taskID int64) string {
	return fmt.Sprintf("%s-%s-%d.log", sanitizeWorkflowFileName(workflowID), logFileNameReplacer.Replace(jobName), taskID)
}

func openTaskLogs(ctx context.Context, task *actions_model.ActionTask) (io.ReadSeekCloser, error) {
	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if errors.Is(err, fs.ErrNotExist) {
		// convert fs err to our own error type so that the API can return a 404 instead of a 500
		return nil, util.NewNotExistErrorf("unable to open task logs: %s", task.LogFilename)
	}
	return reader, err
}

func DownloadActionsRunJobLogsWithID(ctx *context_module.Base, ctxRepo *repo_model.Repository, runID, jobID int64) error {
	job, err := actions_model.GetRunJobByRunAndID(ctx, runID, jobID)
	if err != nil {
		return err
	}
	return DownloadActionsRunJobLogs(ctx, ctxRepo, job)
}

// DownloadActionsRunAllJobLogs assumes the run was already resolved against the requesting repository.
func DownloadActionsRunAllJobLogs(ctx *context_module.Base, run *actions_model.ActionRun) error {
	runJobs, err := actions_model.GetLatestAttemptJobsByRun(ctx, run)
	if err != nil {
		return fmt.Errorf("GetLatestAttemptJobsByRun: %w", err)
	}

	tasks, err := actions_model.GetTasksMapByIDs(ctx, container.FilterSlice(runJobs, func(job *actions_model.ActionRunJob) (int64, bool) {
		taskID := job.EffectiveTaskID()
		return taskID, taskID != 0
	}))
	if err != nil {
		return fmt.Errorf("GetTasksMapByIDs: %w", err)
	}

	// Delay the headers until the first log opens, afterwards a failure can no longer reach
	// the client, so the remaining entries are best-effort.
	var zipWriter *zip.Writer
	for _, job := range runJobs {
		task := tasks[job.EffectiveTaskID()]
		if task == nil || task.LogExpired {
			continue
		}

		reader, err := openTaskLogs(ctx, task)
		if err != nil {
			log.Error("Failed to open logs of job %d: %v", job.ID, err)
			continue
		}

		if zipWriter == nil {
			ctx.SetServeHeaders(context_module.ServeHeaderOptions{
				Filename:           fmt.Sprintf("%s-run-%d-logs.zip", sanitizeWorkflowFileName(run.WorkflowID), run.ID),
				ContentType:        "application/zip",
				ContentDisposition: httplib.ContentDispositionAttachment,
			})
			zipWriter = zip.NewWriter(ctx.Resp)
		}

		zipFile, err := zipWriter.Create(jobLogFileName(run.WorkflowID, job.Name, task.ID))
		if err == nil {
			_, err = io.Copy(zipFile, reader)
		}
		reader.Close()
		if err != nil {
			log.Error("Failed to add logs of job %d to zip: %v", job.ID, err)
		}
	}
	if zipWriter == nil {
		return util.NewNotExistErrorf("logs not found")
	}
	if err := zipWriter.Close(); err != nil {
		log.Error("Failed to finalize logs zip of run %d: %v", run.ID, err)
	}
	return nil
}

func DownloadActionsRunJobLogs(ctx *context_module.Base, ctxRepo *repo_model.Repository, curJob *actions_model.ActionRunJob) error {
	if curJob.RepoID != ctxRepo.ID {
		return util.NewNotExistErrorf("job not found")
	}

	if err := curJob.LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}

	taskID := curJob.EffectiveTaskID()
	if taskID == 0 {
		return util.NewNotExistErrorf("job not started")
	}
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("GetTaskByID: %w", err)
	}
	if task.LogExpired {
		return util.NewNotExistErrorf("logs have been cleaned up")
	}

	reader, err := openTaskLogs(ctx, task)
	if err != nil {
		return err
	}
	defer reader.Close()

	ctx.ServeContent(reader, context_module.ServeHeaderOptions{
		Filename:           jobLogFileName(curJob.Run.WorkflowID, curJob.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain; charset=utf-8",
		ContentDisposition: httplib.ContentDispositionAttachment,
	})
	return nil
}
