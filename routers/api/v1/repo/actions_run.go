// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/routers/common"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
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
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"

	cancelWorkflowRun(ctx, false)
}

func ForceCancelWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/force-cancel repository forceCancelWorkflowRun
	// ---
	// summary: Force-cancel a workflow run
	// description: |
	//   Cancels a workflow run without waiting for its runners to acknowledge the cancellation.
	//   The jobs are marked cancelled at once and anything a runner reports for them afterwards is discarded.
	//   Only use this endpoint when the workflow run does not respond to `POST /repos/{owner}/{repo}/actions/runs/{run}/cancel`.
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
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"

	cancelWorkflowRun(ctx, true)
}

func cancelWorkflowRun(ctx *context.APIContext, force bool) {
	run, jobs := getCurrentRepoActionRunJobsByID(ctx)
	if ctx.Written() {
		return
	}

	// Cancelling a finished run would change nothing, so report a conflict instead of a false success.
	if run.Status.IsDone() {
		ctx.APIError(http.StatusConflict, "run is already completed")
		return
	}

	var err error
	if force {
		run, err = actions_service.ForceCancelRun(ctx, run, jobs)
	} else {
		run, err = actions_service.CancelRun(ctx, run, jobs)
	}
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	respondRepoActionWorkflowRun(ctx, run)
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
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	if !run.NeedApproval {
		// Approving twice is idempotent, but a run that never awaited approval gets 409 rather
		// than GitHub's 403, which would be indistinguishable from a permission denial.
		if run.ApprovedBy == 0 {
			ctx.APIError(http.StatusConflict, "run does not require approval")
			return
		}
		respondRepoActionWorkflowRun(ctx, run)
		return
	}

	approvedRuns, err := actions_service.ApproveRuns(ctx, ctx.Repo.Repository, ctx.Doer, []int64{run.ID})
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	respondRepoActionWorkflowRun(ctx, approvedRuns[0])
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

	if err := common.DownloadActionsRunAllJobLogs(ctx.Base, run); err != nil {
		ctx.APIErrorAuto(err)
	}
}
