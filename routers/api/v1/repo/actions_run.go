// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
)

func getRunID(ctx *context.APIContext) int64 {
	// if run param is "latest", get the latest run index
	if ctx.PathParam("run") == "latest" {
		if run, _ := actions_model.GetLatestRun(ctx, ctx.Repo.Repository.ID); run != nil {
			return run.ID
		}
	}
	return ctx.PathParamInt64("run")
}

func DownloadActionsRunJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/jobs/{job}/logs repository downloadActionsRunJobLogs
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
	// - name: run
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
	//   "200":
	//     description: output blob content
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID := getRunID(ctx)
	jobIndex := ctx.PathParamInt64("job")
	common.DownloadActionsRunJobLogs(ctx.Base, ctx.Repo.Repository, runID, jobIndex)
}
