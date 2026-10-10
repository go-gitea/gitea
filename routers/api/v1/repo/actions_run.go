// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/shared"
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

	if !run.IsAwaitingApproval() {
		// Approving twice is idempotent, but a run that never awaited approval gets 409 rather
		// than GitHub's 403, which would be indistinguishable from a permission denial.
		if run.ApprovedBy == 0 {
			ctx.APIError(http.StatusConflict, "run is not waiting for approval")
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

func getCurrentRepoActionRunByID(ctx *context.APIContext) *actions_model.ActionRun {
	runID := ctx.PathParamInt64("run")
	run, err := actions_model.GetRunByRepoAndID(ctx, ctx.Repo.Repository.ID, runID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	run.Repo = ctx.Repo.Repository
	return run
}

func getCurrentRepoActionRunJobsByID(ctx *context.APIContext) (*actions_model.ActionRun, actions_model.ActionJobList) {
	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return nil, nil
	}

	jobs, err := actions_model.GetLatestAttemptJobsByRun(ctx, run)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil, nil
	}
	jobs.SortMatrixGroupsByName()
	return run, jobs
}

func getCurrentRepoActionRunAttemptByNumber(ctx *context.APIContext) (*actions_model.ActionRun, *actions_model.ActionRunAttempt) {
	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return nil, nil
	}

	attemptNum := ctx.PathParamInt64("attempt")
	attempt, err := actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil, nil
	}
	return run, attempt
}

func respondRepoActionWorkflowRun(ctx *context.APIContext, run *actions_model.ActionRun) {
	run.Repo = ctx.Repo.Repository
	convertedRun, err := convert.ToActionWorkflowRun(ctx, run, nil, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convertedRun)
}

// GetWorkflowRun Gets a specific workflow run.
func GetWorkflowRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run} repository GetWorkflowRun
	// ---
	// summary: Gets a specific workflow run
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
	//   description: id of the run
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	respondRepoActionWorkflowRun(ctx, run)
}

// GetWorkflowRunAttempt Gets a specific workflow run attempt.
func GetWorkflowRunAttempt(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/attempts/{attempt} repository getWorkflowRunAttempt
	// ---
	// summary: Gets a specific workflow run attempt
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
	//   description: id of the run
	//   type: integer
	//   required: true
	// - name: attempt
	//   in: path
	//   description: logical attempt number of the run
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, attempt := getCurrentRepoActionRunAttemptByNumber(ctx)
	if ctx.Written() {
		return
	}

	convertedRun, err := convert.ToActionWorkflowRun(ctx, run, attempt, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convertedRun)
}

// RerunWorkflowRun Reruns an entire workflow run.
func RerunWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/rerun repository rerunWorkflowRun
	// ---
	// summary: Reruns an entire workflow run
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
	//   description: id of the run
	//   type: integer
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/WorkflowRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	run, jobs := getCurrentRepoActionRunJobsByID(ctx)
	if ctx.Written() {
		return
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, jobs); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	convertedRun, err := convert.ToActionWorkflowRun(ctx, run, nil, false)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convertedRun)
}

// RerunFailedWorkflowRun Reruns all failed jobs in a workflow run.
func RerunFailedWorkflowRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/rerun-failed-jobs repository rerunFailedWorkflowRun
	// ---
	// summary: Reruns all failed jobs in a workflow run
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
	//   description: id of the run
	//   type: integer
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	run, jobs := getCurrentRepoActionRunJobsByID(ctx)
	if ctx.Written() {
		return
	}

	failedJobs := actions_service.GetFailedJobsForRerun(jobs)
	// Empty failedJobs means no failed jobs to re-run
	if len(failedJobs) == 0 {
		ctx.APIError(http.StatusBadRequest, "this workflow run has no failed jobs to re-run")
		return
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, failedJobs); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	ctx.Status(http.StatusCreated)
}

// RerunWorkflowJob Reruns a specific workflow job in a run.
func RerunWorkflowJob(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run}/jobs/{job_id}/rerun repository rerunWorkflowJob
	// ---
	// summary: Reruns a specific workflow job in a run
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
	//   description: id of the run
	//   type: integer
	//   required: true
	// - name: job_id
	//   in: path
	//   description: id of the job
	//   type: integer
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/WorkflowJob"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	//   "422":
	//     "$ref": "#/responses/validationError"

	run, jobs := getCurrentRepoActionRunJobsByID(ctx)
	if ctx.Written() {
		return
	}

	jobID := ctx.PathParamInt64("job_id")
	jobIdx := slices.IndexFunc(jobs, func(job *actions_model.ActionRunJob) bool { return job.ID == jobID })
	if jobIdx == -1 {
		ctx.APIErrorNotFound("workflow job not found")
		return
	}

	targetJob := jobs[jobIdx]
	newAttempt, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, []*actions_model.ActionRunJob{targetJob})
	if err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	// Legacy jobs had AttemptJobID=0 before the rerun; createOriginalAttemptForLegacyRun inside
	// RerunWorkflowRunJobs has since backfilled it in the DB, so reload only in that case.
	if targetJob.AttemptJobID == 0 {
		targetJob, err = actions_model.GetRunJobByRepoAndID(ctx, run.RepoID, targetJob.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	rerunJob, err := actions_model.GetRunJobByAttemptJobID(ctx, run.ID, newAttempt.ID, targetJob.AttemptJobID)
	if err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	convertedJob, err := convert.ToActionWorkflowJob(ctx, ctx.Repo.Repository, nil, rerunJob)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convertedJob)
}

func handleWorkflowRerunError(ctx *context.APIContext, err error) {
	if errors.Is(err, util.ErrInvalidArgument) {
		ctx.APIError(http.StatusBadRequest, err.Error())
		return
	} else if errors.Is(err, util.ErrAlreadyExist) {
		ctx.APIError(http.StatusConflict, err.Error())
		return
	} else if errors.Is(err, util.ErrNotExist) {
		ctx.APIError(http.StatusNotFound, err.Error())
		return
	}
	ctx.APIErrorInternal(err)
}

// ListWorkflowRunJobs Lists all jobs for a workflow run.
func ListWorkflowRunJobs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/jobs repository listWorkflowRunJobs
	// ---
	// summary: Lists all jobs for a workflow run
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
	//   description: runid of the workflow run
	//   type: integer
	//   required: true
	// - name: status
	//   in: query
	//   description: workflow status (requested, pending, queued, in_progress, failure, success, skipped)
	//   type: string
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: sort
	//   in: query
	//   description: sort jobs by attribute. Supported values are "id". Default is "id"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending). Default is "asc"
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowJobsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	repoID, runID := ctx.Repo.Repository.ID, ctx.PathParamInt64("run")

	// Avoid the list all jobs functionality for this api route to be used with a runID == 0.
	if runID <= 0 {
		ctx.APIError(http.StatusBadRequest, "runID must be a positive integer")
		return
	}

	run, err := actions_model.GetRunByRepoAndID(ctx, repoID, runID)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	// runID is used as an additional filter next to repoID to ensure that we only list jobs for the specified repoID and runID.
	// no additional checks for runID are needed here
	shared.ListJobs(ctx, 0, repoID, runID, optional.Some(run.LatestAttemptID))
}

// ListWorkflowRunAttemptJobs Lists all jobs for a workflow run attempt.
func ListWorkflowRunAttemptJobs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run}/attempts/{attempt}/jobs repository listWorkflowRunAttemptJobs
	// ---
	// summary: Lists all jobs for a workflow run attempt
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
	//   description: id of the workflow run
	//   type: integer
	//   required: true
	// - name: attempt
	//   in: path
	//   description: logical attempt number of the run
	//   type: integer
	//   required: true
	// - name: status
	//   in: query
	//   description: workflow status (requested, pending, queued, in_progress, failure, success, skipped)
	//   type: string
	//   required: false
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowJobsList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, attempt := getCurrentRepoActionRunAttemptByNumber(ctx)
	if ctx.Written() {
		return
	}

	shared.ListJobs(ctx, 0, run.RepoID, run.ID, optional.Some(attempt.ID))
}

// GetWorkflowJob Gets a specific workflow job for a workflow run.
func GetWorkflowJob(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs/{job_id} repository getWorkflowJob
	// ---
	// summary: Gets a specific workflow job for a workflow run
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
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/WorkflowJob"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	jobID := ctx.PathParamInt64("job_id")
	job, has, err := db.GetByID[actions_model.ActionRunJob](ctx, jobID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	if !has || job.RepoID != ctx.Repo.Repository.ID {
		ctx.APIErrorNotFound()
		return
	}

	convertedWorkflowJob, err := convert.ToActionWorkflowJob(ctx, ctx.Repo.Repository, nil, job)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convertedWorkflowJob)
}

// DeleteActionRun Delete a workflow run
func DeleteActionRun(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/runs/{run} repository deleteActionRun
	// ---
	// summary: Delete a workflow run
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
	//   description: runid of the workflow run
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     description: "No Content"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run := getCurrentRepoActionRunByID(ctx)
	if ctx.Written() {
		return
	}

	if !run.Status.IsDone() {
		ctx.APIError(http.StatusBadRequest, "this workflow run is not done")
		return
	}

	if err := actions_service.DeleteRun(ctx, run); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
