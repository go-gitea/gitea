// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	"gitea.dev/models/unit"
	"gitea.dev/modules/util"
	actions_service "gitea.dev/services/actions"
	context_module "gitea.dev/services/context"
)

// checkRunRerunAllowed checks whether a rerun is permitted for the given run,
// writing the appropriate JSON error to ctx and returning false when it is not.
func checkRunRerunAllowed(ctx *context_module.Context, run *actions_model.ActionRun) bool {
	if !run.Status.IsDone() {
		ctx.JSONError(ctx.Locale.Tr("actions.runs.not_done"))
		return false
	}
	cfgUnit := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypeActions)
	cfg := cfgUnit.ActionsConfig()
	disabled := cfg.IsWorkflowDisabled(run.WorkflowID)
	if run.IsScopedRun {
		optedOut, err := actions_model.IsScopedWorkflowOptedOut(ctx, cfg, ctx.Repo.Repository.OwnerID, run.WorkflowRepoID, run.WorkflowID)
		if err != nil {
			ctx.ServerError("IsScopedWorkflowOptedOut", err)
			return false
		}
		disabled = optedOut
	}
	if disabled {
		ctx.JSONError(ctx.Locale.Tr("actions.workflow.disabled"))
		return false
	}
	return true
}

func checkLatestAttempt(ctx *context_module.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt) bool {
	if attempt != nil && run.LatestAttemptID != attempt.ID {
		ctx.NotFound(nil)
		return false
	}
	return true
}

// Rerun will rerun jobs in the given run
// If jobIDStr is a blank string, it means rerun all jobs
func Rerun(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}
	if !checkRunRerunAllowed(ctx, run) {
		return
	}
	if len(jobs) == 0 {
		ctx.JSONError(ctx.Locale.Tr("actions.runs.no_job"))
		return
	}

	currentJob, hasPathParam := findCurrentJobByPathParam(ctx, jobs)
	if hasPathParam && currentJob == nil {
		ctx.NotFound(nil)
		return
	}

	var jobsToRerun []*actions_model.ActionRunJob
	if currentJob != nil {
		jobsToRerun = []*actions_model.ActionRunJob{currentJob}
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, jobsToRerun); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	ctx.JSONRedirect(run.Link())
}

// RerunFailed reruns all failed jobs in the given run
func RerunFailed(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}
	if !checkRunRerunAllowed(ctx, run) {
		return
	}

	// An empty job list means "re-run the whole run" to RerunWorkflowRunJobs, which is right for the plain
	// rerun button but wrong here, so a direct POST on a fully successful run cannot re-run everything.
	failedJobs := actions_service.GetFailedJobsForRerun(jobs)
	if len(failedJobs) == 0 {
		ctx.JSONError(ctx.Locale.Tr("actions.runs.no_failed_jobs"))
		return
	}

	if _, err := actions_service.RerunWorkflowRunJobs(ctx, ctx.Repo.Repository, run, ctx.Doer, failedJobs); err != nil {
		handleWorkflowRerunError(ctx, err)
		return
	}

	ctx.JSONRedirect(run.Link())
}

func handleWorkflowRerunError(ctx *context_module.Context, err error) {
	if errors.Is(err, util.ErrAlreadyExist) {
		ctx.JSON(http.StatusConflict, map[string]any{"message": err.Error()})
		return
	}
	if errors.Is(err, util.ErrInvalidArgument) {
		ctx.JSON(http.StatusBadRequest, map[string]any{"message": err.Error()})
		return
	}
	ctx.ServerError("RerunWorkflowRunJobs", err)
}

func Cancel(ctx *context_module.Context) {
	run, attempt, jobs := getCurrentRunJobsByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if !checkLatestAttempt(ctx, run, attempt) {
		return
	}

	if _, err := actions_service.CancelRun(ctx, run, jobs); err != nil {
		ctx.ServerError("CancelRun", err)
		return
	}
	ctx.JSONOK()
}

func Approve(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	if _, err := actions_service.ApproveRuns(ctx, ctx.Repo.Repository, ctx.Doer, []int64{run.ID}); err != nil {
		ctx.NotFoundOrServerError("ApproveRuns", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}

	ctx.JSONOK()
}

func Delete(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}

	if !run.Status.IsDone() {
		ctx.JSONError(ctx.Tr("actions.runs.not_done"))
		return
	}

	if err := actions_service.DeleteRun(ctx, run); err != nil {
		ctx.ServerError("DeleteRun", err)
		return
	}

	ctx.JSONOK()
}

func ApproveAllChecks(ctx *context_module.Context) {
	repo := ctx.Repo.Repository
	commitID := ctx.FormString("commit_id")

	commitStatuses, err := git_model.GetLatestCommitStatus(ctx, repo.ID, commitID, db.ListOptionsAll)
	if err != nil {
		ctx.ServerError("GetLatestCommitStatus", err)
		return
	}
	runs, err := actions_service.GetRunsFromCommitStatuses(ctx, commitStatuses)
	if err != nil {
		ctx.ServerError("GetRunsFromCommitStatuses", err)
		return
	}

	runIDs := make([]int64, 0, len(runs))
	for _, run := range runs {
		if run.IsAwaitingApproval() {
			runIDs = append(runIDs, run.ID)
		}
	}

	if len(runIDs) == 0 {
		ctx.JSONOK()
		return
	}

	if _, err := actions_service.ApproveRuns(ctx, repo, ctx.Doer, runIDs); err != nil {
		ctx.NotFoundOrServerError("ApproveRuns", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}

	ctx.Flash.Success(ctx.Tr("actions.approve_all_success"))
	ctx.JSONOK()
}
