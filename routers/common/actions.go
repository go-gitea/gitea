// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/services/context"
)

func DownloadActionsRunJobLogs(ctx *context.Base, ctxRepo *repo_model.Repository, runID, jobIndex int64) {
	if runID == 0 {
		ctx.HTTPError(http.StatusBadRequest, "invalid run id")
		return
	}

	runJobs, err := actions_model.GetRunJobsByRunID(ctx, runID)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}
	if len(runJobs) == 0 {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	if err := runJobs.LoadRepos(ctx); err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}
	if runJobs[0].Repo.ID != ctxRepo.ID {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	var curJob *actions_model.ActionRunJob
	for _, job := range runJobs {
		if job.ID == jobIndex {
			curJob = job
			break
		}
	}
	if curJob == nil {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	if curJob.TaskID == 0 {
		ctx.HTTPError(http.StatusNotFound, "job is not started")
		return
	}

	if err := curJob.LoadRun(ctx); err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}

	task, err := actions_model.GetTaskByID(ctx, curJob.TaskID)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}
	if task.LogExpired {
		ctx.HTTPError(http.StatusNotFound, "logs have been cleaned up")
		return
	}

	reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
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
}
