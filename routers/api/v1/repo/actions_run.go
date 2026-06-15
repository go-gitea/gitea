// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/routers/common"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func DownloadActionsRunJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs repository downloadActionsRunJobLogs
	// ---
	// summary: Downloads the job logs for a workflow run
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: job_id
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

	jobID := ctx.PathParamInt64("job_id")
	curJob, err := actions_model.GetRunJobByRepoAndID(ctx, ctx.Repo.Repository.ID, jobID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if err = curJob.LoadRepo(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	err = common.DownloadActionsRunJobLogs(ctx.Base, ctx.Repo.Repository, curJob)
	if err != nil {
		ctx.APIErrorAuto(err)
	}
}

func CancelWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/cancel repository cancelWorkflowRun
	// ---
	// summary: Cancel a workflow run and its jobs
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, jobs := getCurrentRepoActionRunJobsByID(ctx)
	if ctx.Written() {
		return
	}

	if err := actions_service.CancelRun(ctx, run, jobs); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	updatedRun, has, err := db.GetByID[actions_model.ActionRun](ctx, run.ID)
	if err != nil || !has {
		ctx.APIErrorInternal(err)
		return
	}

	updatedRun.Repo = ctx.Repo.Repository
	respondActionWorkflowRun(ctx, updatedRun)
}

func ApproveWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/approve repository approveWorkflowRun
	// ---
	// summary: Approve a workflow run that requires approval
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: success
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	// GitHub-compatible: return 200 if already approved (idempotent)
	if !run.NeedApproval {
		respondActionWorkflowRun(ctx, run)
		return
	}

	if err := actions_service.ApproveRuns(ctx, ctx.Repo.Repository, ctx.Doer, []int64{run.ID}); err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	// Note: the overall run status is updated asynchronously by the notifier,
	// so the status field may still reflect the pre-approval state.
	run.NeedApproval = false
	run.ApprovedBy = ctx.Doer.ID
	respondActionWorkflowRun(ctx, run)
}

func respondActionWorkflowRun(ctx *context.APIContext, run *actions_model.ActionRun) {
	run.Repo = ctx.Repo.Repository
	convertedRun, err := convert.ToActionWorkflowRun(ctx, run, nil, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convertedRun)
}

func GetWorkflowRunLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/logs repository getWorkflowRunLogs
	// ---
	// summary: Download workflow run logs as archive
	// produces:
	// - application/zip
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: Logs archive
	//   "404":
	//     "$ref": "#/responses/notFound"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	if err := common.DownloadActionsRunAllJobLogs(ctx.Base, ctx.Repo.Repository, run.ID); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
}

func GetWorkflowJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/jobs/{job_id}/logs repository getWorkflowJobLogs
	// ---
	// summary: Download job logs as plain text
	// produces:
	// - text/plain
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: run
	//   in: path
	//   description: run ID
	//   type: integer
	//   required: true
	// - name: job_id
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     description: Job logs
	//   "404":
	//     "$ref": "#/responses/notFound"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	jobID := ctx.PathParamInt64("job_id")
	if err := common.DownloadActionsRunJobLogsWithID(ctx.Base, ctx.Repo.Repository, run.ID, jobID); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
}
