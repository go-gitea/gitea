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
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	context_module "gitea.dev/services/context"
)

var (
	workflowNameReplacer = strings.NewReplacer(`"`, "", "\r", "", "\n", "", "/", "-", `\`, "-")
	jobNameReplacer      = strings.NewReplacer("/", "-", `\`, "-", "..", "__")
)

func sanitizeWorkflowFileName(workflowID string) string {
	if p := strings.Index(workflowID, "."); p > 0 {
		workflowID = workflowID[:p]
	}
	return workflowNameReplacer.Replace(workflowID)
}

func sanitizeJobFileName(name string) string {
	return jobNameReplacer.Replace(name)
}

func jobLogFileName(workflowID, jobName string, taskID int64) string {
	return fmt.Sprintf("%s-%s-%d.log", sanitizeWorkflowFileName(workflowID), sanitizeJobFileName(jobName), taskID)
}

func resolveJobLogTask(ctx context.Context, job *actions_model.ActionRunJob) (*actions_model.ActionTask, error) {
	taskID := job.EffectiveTaskID()
	if taskID == 0 {
		return nil, util.NewNotExistErrorf("job not started")
	}

	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("GetTaskByID: %w", err)
	}

	if task.LogExpired {
		return nil, util.NewNotExistErrorf("logs have been cleaned up")
	}
	if task.LogLength == 0 {
		return nil, util.NewNotExistErrorf("logs not found")
	}
	return task, nil
}

func openTaskLogs(ctx context.Context, task *actions_model.ActionTask) (io.ReadSeekCloser, error) {
	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, util.ErrNotExist) {
			return nil, util.NewNotExistErrorf("logs not found")
		}
		return nil, fmt.Errorf("OpenLogs: %w", err)
	}
	return reader, nil
}

func openJobTaskLogs(ctx context.Context, job *actions_model.ActionRunJob) (io.ReadSeekCloser, *actions_model.ActionTask, error) {
	task, err := resolveJobLogTask(ctx, job)
	if err != nil {
		return nil, nil, err
	}

	reader, err := openTaskLogs(ctx, task)
	if err != nil {
		return nil, nil, err
	}
	return reader, task, nil
}

func appendJobLogToZip(ctx context.Context, zipWriter *zip.Writer, workflowID string, job *actions_model.ActionRunJob, task *actions_model.ActionTask) error {
	reader, err := openTaskLogs(ctx, task)
	if err != nil {
		return err
	}
	defer reader.Close()

	zipFile, err := zipWriter.Create(jobLogFileName(workflowID, job.Name, task.ID))
	if err != nil {
		return fmt.Errorf("Create zip entry for job %d: %w", job.ID, err)
	}
	if _, err = io.Copy(zipFile, reader); err != nil {
		return fmt.Errorf("Write job %d logs to zip: %w", job.ID, err)
	}
	return nil
}

func DownloadActionsRunJobLogsWithID(ctx *context_module.Base, ctxRepo *repo_model.Repository, runID, jobID int64) error {
	job, err := actions_model.GetRunJobByRunAndID(ctx, runID, jobID)
	if err != nil {
		return err
	}
	return DownloadActionsRunJobLogs(ctx, ctxRepo, job)
}

func DownloadActionsRunAllJobLogs(ctx *context_module.Base, ctxRepo *repo_model.Repository, runID int64) error {
	runJobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, ctxRepo.ID, runID)
	if err != nil {
		return fmt.Errorf("GetLatestAttemptJobsByRepoAndRunID: %w", err)
	}

	if len(runJobs) == 0 {
		return util.NewNotExistErrorf("no jobs found for run %d", runID)
	}

	if err := runJobs[0].LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}
	workflowID := runJobs[0].Run.WorkflowID

	type jobLogEntry struct {
		job  *actions_model.ActionRunJob
		task *actions_model.ActionTask
	}
	logEntries := make([]jobLogEntry, 0, len(runJobs))
	for _, job := range runJobs {
		task, err := resolveJobLogTask(ctx, job)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				continue
			}
			return err
		}
		logEntries = append(logEntries, jobLogEntry{job: job, task: task})
	}
	if len(logEntries) == 0 {
		return util.NewNotExistErrorf("logs not found")
	}

	ctx.Resp.Header().Set("Content-Type", "application/zip")
	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(
		fmt.Sprintf("%s-run-%d-logs.zip", sanitizeWorkflowFileName(workflowID), runID),
	))

	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	// Best-effort: the response headers and zip stream are already committed, so a
	// failure to read one job's logs must not abort the whole archive. Log and skip.
	for _, entry := range logEntries {
		if err := appendJobLogToZip(ctx, zipWriter, workflowID, entry.job, entry.task); err != nil {
			log.Error("Failed to add logs for job %d to zip: %v", entry.job.ID, err)
			continue
		}
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

	reader, task, err := openJobTaskLogs(ctx, curJob)
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
