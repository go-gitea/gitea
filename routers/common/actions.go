// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"archive/zip"
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
	"gitea.dev/services/context"
)

func actionsWorkflowBaseName(workflowID string) string {
	if p := strings.Index(workflowID, "."); p > 0 {
		return workflowID[:p]
	}
	return workflowID
}

func DownloadActionsRunJobLogsWithID(ctx *context.Base, ctxRepo *repo_model.Repository, runID, jobID int64) error {
	job, err := actions_model.GetRunJobByRunAndID(ctx, runID, jobID)
	if err != nil {
		return err
	}
	if err := job.LoadRepo(ctx); err != nil {
		return fmt.Errorf("LoadRepo: %w", err)
	}
	return DownloadActionsRunJobLogs(ctx, ctxRepo, job)
}

func DownloadActionsRunAllJobLogs(ctx *context.Base, ctxRepo *repo_model.Repository, runID int64) error {
	runJobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, ctxRepo.ID, runID)
	if err != nil {
		return fmt.Errorf("GetLatestAttemptJobsByRepoAndRunID: %w", err)
	}

	if len(runJobs) == 0 {
		return util.NewNotExistErrorf("no jobs found for run %d", runID)
	}

	// Load run for workflow name
	if err := runJobs[0].LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}

	workflowName := actionsWorkflowBaseName(runJobs[0].Run.WorkflowID)
	safeWorkflowName := strings.NewReplacer(`"`, "", "\r", "", "\n", "", "/", "-", `\`, "-").Replace(workflowName)

	ctx.Resp.Header().Set("Content-Type", "application/zip")
	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(fmt.Sprintf("%s-run-%d-logs.zip", safeWorkflowName, runID)))

	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	jobNameReplacer := strings.NewReplacer("/", "-", `\`, "-", "..", "__")
	for _, job := range runJobs {
		taskID := job.EffectiveTaskID()
		if taskID == 0 {
			continue // Skip jobs that haven't started
		}

		task, err := actions_model.GetTaskByID(ctx, taskID)
		if err != nil {
			return fmt.Errorf("GetTaskByID for job %d: %w", job.ID, err)
		}

		if task.LogExpired || task.LogLength == 0 {
			continue
		}

		safeJobName := jobNameReplacer.Replace(job.Name)
		fileName := fmt.Sprintf("%s-%s-%d.log", safeWorkflowName, safeJobName, task.ID)

		reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			log.Error("Failed to open logs for job %d: %v", job.ID, err)
			continue
		}

		zipFile, err := zipWriter.Create(fileName)
		if err != nil {
			reader.Close()
			log.Error("Failed to add logs for job %d to zip: %v", job.ID, err)
			continue
		}

		if _, err = io.Copy(zipFile, reader); err != nil {
			log.Error("Failed to add logs for job %d to zip: %v", job.ID, err)
		}
		reader.Close()
	}

	return nil
}

func DownloadActionsRunJobLogs(ctx *context.Base, ctxRepo *repo_model.Repository, curJob *actions_model.ActionRunJob) error {
	if curJob.Repo.ID != ctxRepo.ID {
		return util.NewNotExistErrorf("job not found")
	}

	taskID := curJob.EffectiveTaskID()
	if taskID == 0 {
		return util.NewNotExistErrorf("job not started")
	}

	if err := curJob.LoadRun(ctx); err != nil {
		return fmt.Errorf("LoadRun: %w", err)
	}

	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("GetTaskByID: %w", err)
	}

	if task.LogExpired {
		return util.NewNotExistErrorf("logs have been cleaned up")
	}

	if task.LogLength == 0 {
		return util.NewNotExistErrorf("logs not found")
	}

	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, util.ErrNotExist) {
			return util.NewNotExistErrorf("logs not found")
		}
		return fmt.Errorf("OpenLogs: %w", err)
	}
	defer reader.Close()

	workflowName := actionsWorkflowBaseName(curJob.Run.WorkflowID)
	ctx.ServeContent(reader, context.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%v-%v-%v.log", workflowName, curJob.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain; charset=utf-8",
		ContentDisposition: httplib.ContentDispositionAttachment,
	})
	return nil
}
