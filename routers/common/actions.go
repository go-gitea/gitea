// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

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
	ctx.ServeContent(reader, context.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%v-%v-%v.log", workflowName, curJob.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain; charset=utf-8",
		ContentDisposition: httplib.ContentDispositionAttachment,
	})
	return nil
}
