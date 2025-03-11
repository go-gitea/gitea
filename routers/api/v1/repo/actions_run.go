// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

func getRunIndex(ctx *context.APIContext) int64 {
	// if run param is "latest", get the latest run index
	if ctx.PathParam("run_id") == "latest" {
		if run, _ := actions_model.GetLatestRun(ctx, ctx.Repo.Repository.ID); run != nil {
			return run.Index
		}
	}
	return ctx.PathParamInt64("run_id")
}

// getRunJobs gets the jobs of runIndex, and returns jobs[jobIndex], jobs.
// Any error will be written to the ctx.
// It never returns a nil job of an empty jobs, if the jobIndex is out of range, it will be treated as 0.
func getRunJobs(ctx *context.APIContext, runIndex, jobIndex int64) (*actions_model.ActionRunJob, []*actions_model.ActionRunJob) {
	run, err := actions_model.GetRunByIndex(ctx, ctx.Repo.Repository.ID, runIndex)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.HTTPError(http.StatusNotFound, err.Error())
			return nil, nil
		}
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return nil, nil
	}
	run.Repo = ctx.Repo.Repository
	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return nil, nil
	}
	if len(jobs) == 0 {
		ctx.HTTPError(http.StatusNotFound)
		return nil, nil
	}

	for _, v := range jobs {
		v.Run = run
	}

	if jobIndex >= 0 && jobIndex < int64(len(jobs)) {
		return jobs[jobIndex], jobs
	}
	return jobs[0], jobs
}

func DownloadActionsRunLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs/{job}/logs repository downloadActionsRunLogs
	// ---
	// summary: Downloads the logs for a workflow run redirects to blob url
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run_id
	//   in: path
	//   description: id of the run, this could be latest
	//   type: integer
	//   required: true
	// - name: job
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "302":
	//     description: redirect to the blob download
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runIndex := getRunIndex(ctx)
	jobIndex := ctx.PathParamInt64("job")

	job, _ := getRunJobs(ctx, runIndex, jobIndex)
	if ctx.Written() {
		return
	}
	if job.TaskID == 0 {
		ctx.HTTPError(http.StatusNotFound, "job is not started")
		return
	}

	err := job.LoadRun(ctx)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, err.Error())
		return
	}

	task, err := actions_model.GetTaskByID(ctx, job.TaskID)
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

	workflowName := job.Run.WorkflowID
	if p := strings.Index(workflowName, "."); p > 0 {
		workflowName = workflowName[0:p]
	}
	ctx.ServeContent(reader, &context.ServeHeaderOptions{
		Filename:           fmt.Sprintf("%v-%v-%v.log", workflowName, job.Name, task.ID),
		ContentLength:      &task.LogSize,
		ContentType:        "text/plain",
		ContentTypeCharset: "utf-8",
		Disposition:        "attachment",
	})
}
