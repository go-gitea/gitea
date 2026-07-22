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

// logFileNameReplacer keeps a workflow or job name usable as a file name: it drops
// path separators and traversal sequences so the result stays a single zip entry,
// and drops the characters that would otherwise survive into a download filename.
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

type jobLogEntry struct {
	job  *actions_model.ActionRunJob
	task *actions_model.ActionTask
}

// collectJobLogEntries pairs each job with the task holding its logs, skipping jobs
// that never started or whose logs are already gone. The tasks are fetched in one query.
func collectJobLogEntries(ctx context.Context, jobs []*actions_model.ActionRunJob) ([]jobLogEntry, error) {
	taskIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if taskID := job.EffectiveTaskID(); taskID != 0 {
			taskIDs = append(taskIDs, taskID)
		}
	}

	tasks, err := actions_model.GetTasksByIDs(ctx, taskIDs)
	if err != nil {
		return nil, fmt.Errorf("GetTasksByIDs: %w", err)
	}

	entries := make([]jobLogEntry, 0, len(jobs))
	for _, job := range jobs {
		task := tasks[job.EffectiveTaskID()]
		if task == nil || task.LogExpired {
			continue
		}
		entries = append(entries, jobLogEntry{job: job, task: task})
	}
	return entries, nil
}

func writeJobLogToZip(zipWriter *zip.Writer, workflowID string, entry jobLogEntry, reader io.Reader) error {
	zipFile, err := zipWriter.Create(jobLogFileName(workflowID, entry.job.Name, entry.task.ID))
	if err != nil {
		return fmt.Errorf("create zip entry for job %d: %w", entry.job.ID, err)
	}
	if _, err = io.Copy(zipFile, reader); err != nil {
		return fmt.Errorf("write job %d logs to zip: %w", entry.job.ID, err)
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

// DownloadActionsRunAllJobLogs streams the logs of the run's latest attempt as a zip archive.
// The run must already be resolved against the requesting repository.
func DownloadActionsRunAllJobLogs(ctx *context_module.Base, run *actions_model.ActionRun) error {
	runJobs, err := actions_model.GetLatestAttemptJobsByRun(ctx, run)
	if err != nil {
		return fmt.Errorf("GetLatestAttemptJobsByRun: %w", err)
	}

	entries, err := collectJobLogEntries(ctx, runJobs)
	if err != nil {
		return err
	}

	// Open the first readable log before writing anything: once the response headers and
	// the zip stream are committed, a failure can no longer be reported to the client.
	firstIdx := -1
	var firstReader io.ReadSeekCloser
	for i, entry := range entries {
		reader, err := openTaskLogs(ctx, entry.task)
		if err != nil {
			log.Error("Failed to open logs of job %d: %v", entry.job.ID, err)
			continue
		}
		firstIdx, firstReader = i, reader
		break
	}
	if firstReader == nil {
		return util.NewNotExistErrorf("logs not found")
	}
	defer firstReader.Close()

	ctx.SetServeHeaders(context_module.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%s-run-%d-logs.zip", sanitizeWorkflowFileName(run.WorkflowID), run.ID),
		ContentType:        "application/zip",
		ContentDisposition: httplib.ContentDispositionAttachment,
	})

	zipWriter := zip.NewWriter(ctx.Resp)
	defer func() {
		if err := zipWriter.Close(); err != nil {
			log.Error("Failed to finalize logs zip of run %d: %v", run.ID, err)
		}
	}()

	// Best-effort from here on: the response is already committed, so a failure to read
	// one job's logs must not abort the whole archive. Log and skip.
	if err := writeJobLogToZip(zipWriter, run.WorkflowID, entries[firstIdx], firstReader); err != nil {
		log.Error("Failed to add logs of job %d to zip: %v", entries[firstIdx].job.ID, err)
	}
	for _, entry := range entries[firstIdx+1:] {
		reader, err := openTaskLogs(ctx, entry.task)
		if err != nil {
			log.Error("Failed to open logs of job %d: %v", entry.job.ID, err)
			continue
		}
		err = writeJobLogToZip(zipWriter, run.WorkflowID, entry, reader)
		reader.Close()
		if err != nil {
			log.Error("Failed to add logs of job %d to zip: %v", entry.job.ID, err)
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
