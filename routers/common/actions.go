// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
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
	if 0 < jobIndex || jobIndex >= int64(len(runJobs)) {
		return util.NewNotExistErrorf("job index is out of range: %d", jobIndex)
	}
	return DownloadActionsRunJobLogs(ctx, ctxRepo, runJobs[jobIndex])
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
