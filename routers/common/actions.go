// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

func DownloadActionsRunJobLogsWithIndex(ctx *context.Base, ctxRepo *repo_model.Repository, runID, jobIndex int64) error {
	runJobs, err := actions_model.GetRunJobsByRunID(ctx, runID)
	if err != nil {
		return fmt.Errorf("GetRunJobsByRunID: %w", err)
	}
	if err = runJobs.LoadRepos(ctx); err != nil {
		return fmt.Errorf("LoadRepos: %w", err)
	}
	if jobIndex < 0 || jobIndex >= int64(len(runJobs)) {
		return util.NewNotExistErrorf("job index is out of range: %d", jobIndex)
	}
	return DownloadActionsRunJobLogs(ctx, ctxRepo, runJobs[jobIndex])
}

func DownloadActionsRunAllJobLogs(ctx *context.Base, ctxRepo *repo_model.Repository, runID int64) error {
	runJobs, err := actions_model.GetRunJobsByRunID(ctx, runID)
	if err != nil {
		return fmt.Errorf("GetRunJobsByRunID: %w", err)
	}
	if err = runJobs.LoadRepos(ctx); err != nil {
		return fmt.Errorf("LoadRepos: %w", err)
	}

	if len(runJobs) == 0 {
		return util.NewNotExistErrorf("no jobs found for run %d", runID)
	}

	// Load run for workflow name
	if err := runJobs[0].LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}

	workflowName := runJobs[0].Run.WorkflowID
	if p := strings.Index(workflowName, "."); p > 0 {
		workflowName = workflowName[0:p]
	}

	// Set headers for zip download
	ctx.Resp.Header().Set("Content-Type", "application/zip")
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-run-%d-logs.zip\"", workflowName, runID))

	// Create zip writer
	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	// Add each job's logs to the zip
	for _, job := range runJobs {
		if job.Repo.ID != ctxRepo.ID {
			continue // Skip jobs from other repos
		}

		if job.TaskID == 0 {
			continue // Skip jobs that haven't started
		}

		task, err := actions_model.GetTaskByID(ctx, job.TaskID)
		if err != nil {
			return fmt.Errorf("GetTaskByID for job %d: %w", job.ID, err)
		}

		if task.LogExpired {
			continue // Skip expired logs
		}

		reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			return fmt.Errorf("OpenLogs for job %d: %w", job.ID, err)
		}

		// Create file in zip with job name and task ID
		fileName := fmt.Sprintf("%s-%s-%d.log", workflowName, job.Name, task.ID)
		zipFile, err := zipWriter.Create(fileName)
		if err != nil {
			reader.Close()
			return fmt.Errorf("Create zip file %s: %w", fileName, err)
		}

		// Copy log content to zip file
		if _, err := io.Copy(zipFile, reader); err != nil {
			reader.Close()
			return fmt.Errorf("Copy logs for job %d: %w", job.ID, err)
		}

		reader.Close()
	}

	return nil
}

func DownloadActionsRunJobLogs(ctx *context.Base, ctxRepo *repo_model.Repository, curJob *actions_model.ActionRunJob) error {
	if curJob.Repo.ID != ctxRepo.ID {
		return util.NewNotExistErrorf("job not found")
	}

	if curJob.TaskID == 0 {
		return util.NewNotExistErrorf("job not started")
	}

	if err := curJob.LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}

	task, err := actions_model.GetTaskByID(ctx, curJob.TaskID)
	if err != nil {
		return fmt.Errorf("GetTaskByID: %w", err)
	}

	if task.LogExpired {
		return util.NewNotExistErrorf("logs have been cleaned up")
	}

	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if err != nil {
		return fmt.Errorf("OpenLogs: %w", err)
	}
	defer reader.Close()

	workflowName := curJob.Run.WorkflowID
	if p := strings.Index(workflowName, "."); p > 0 {
		workflowName = workflowName[0:p]
	}
	ctx.ServeContent(reader, &context.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%v-%v-%v.log", workflowName, curJob.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain",
		ContentTypeCharset: "utf-8",
		Disposition:        "attachment",
	})
	return nil
}
