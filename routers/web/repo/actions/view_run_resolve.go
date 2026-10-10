// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/util"
	context_module "gitea.dev/services/context"
)

func findCurrentJobByPathParam(ctx *context_module.Context, jobs []*actions_model.ActionRunJob) (job *actions_model.ActionRunJob, hasPathParam bool) {
	selectedJobID := ctx.PathParamInt64("job")
	if selectedJobID <= 0 {
		return nil, false
	}
	for _, job = range jobs {
		if job.ID == selectedJobID {
			return job, true
		}
	}
	return nil, true
}

func getCurrentRunByPathParam(ctx *context_module.Context) (run *actions_model.ActionRun) {
	var err error
	// if run param is "latest", get the latest run id
	if ctx.PathParam("run") == "latest" {
		run, err = actions_model.GetLatestRun(ctx, ctx.Repo.Repository.ID)
	} else {
		run, err = actions_model.GetRunByRepoAndID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("run"))
	}
	if errors.Is(err, util.ErrNotExist) {
		ctx.NotFound(nil)
	} else if err != nil {
		ctx.ServerError("GetRun:"+ctx.PathParam("run"), err)
	}
	return run
}

// resolveCurrentRunForView resolves GET Actions page URLs and supports both ID-based and legacy index-based forms.
//
// By default, run summary pages (/actions/runs/{run}) use a best-effort ID-first fallback,
// and job pages (/actions/runs/{run}/jobs/{job}) try to confirm an ID-based URL first and prefer the ID-based interpretation when both are valid.
//
// `by_id=1` param explicitly forces the ID-based path, and `by_index=1` explicitly forces the legacy index-based path.
// If both are present, `by_id` takes precedence.
func resolveCurrentRunForView(ctx *context_module.Context) *actions_model.ActionRun {
	// `by_id` explicitly requests ID-based resolution, so the request skips the legacy index-based disambiguation logic and resolves the run by ID directly.
	// It takes precedence over `by_index` when both query parameters are present.
	if ctx.PathParam("run") == "latest" || ctx.FormBool("by_id") {
		return getCurrentRunByPathParam(ctx)
	}

	runNum := ctx.PathParamInt64("run")
	if runNum <= 0 {
		ctx.NotFound(nil)
		return nil
	}

	byIndex := ctx.FormBool("by_index")

	if ctx.PathParam("job") == "" {
		// The URL does not contain a {job} path parameter, so it cannot use the
		// job-specific rules to disambiguate ID-based URLs from legacy index-based URLs.
		// Because of that, this path is handled with a best-effort ID-first fallback by default.
		//
		// When the same repository contains:
		//  - a run whose ID matches runNum, and
		//  - a different run whose repo-scope index also matches runNum
		// this path prefers the ID match and may show a different run than the old legacy URL originally intended,
		// unless `by_index=1` explicitly forces the legacy index-based interpretation.

		if !byIndex {
			runByID, err := actions_model.GetRunByRepoAndID(ctx, ctx.Repo.Repository.ID, runNum)
			if err == nil {
				return runByID
			}
			if !errors.Is(err, util.ErrNotExist) {
				ctx.ServerError("GetRun:"+ctx.PathParam("run"), err)
				return nil
			}
		}

		runByIndex, err := actions_model.GetRunByRepoAndIndex(ctx, ctx.Repo.Repository.ID, runNum)
		if err == nil {
			ctx.Redirect(fmt.Sprintf("%s/actions/runs/%d", ctx.Repo.RepoLink, runByIndex.ID), http.StatusFound)
			return nil
		}
		if !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunByRepoAndIndex", err)
			return nil
		}
		ctx.NotFound(nil)
		return nil
	}

	jobNum := ctx.PathParamInt64("job")
	if jobNum < 0 {
		ctx.NotFound(nil)
		return nil
	}

	// A job index should not be larger than MaxJobNumPerRun, so larger values can skip the legacy index-based path and be treated as job IDs directly.
	if !byIndex && jobNum >= actions_model.MaxJobNumPerRun {
		return getCurrentRunByPathParam(ctx)
	}

	var runByID, runByIndex *actions_model.ActionRun
	var targetJobByIndex *actions_model.ActionRunJob

	if !byIndex {
		// Probe the repo-scoped job ID first and only accept it when the job exists and belongs to the same runNum.
		job, err := actions_model.GetRunJobByRepoAndID(ctx, ctx.Repo.Repository.ID, jobNum)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunJobByRepoAndID", err)
			return nil
		}
		if job != nil {
			if err := job.LoadRun(ctx); err != nil {
				ctx.ServerError("LoadRun", err)
				return nil
			}
			if job.Run.ID == runNum {
				runByID = job.Run
			}
		}
	}

	// Try to resolve the request as a legacy run-index/job-index URL.
	{
		run, err := actions_model.GetRunByRepoAndIndex(ctx, ctx.Repo.Repository.ID, runNum)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			ctx.ServerError("GetRunByRepoAndIndex", err)
			return nil
		}
		if run != nil {
			jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(ctx, run.RepoID, run.ID)
			if err != nil {
				ctx.ServerError("GetRunJobsByRunID", err)
				return nil
			}
			if jobNum < int64(len(jobs)) {
				runByIndex = run
				targetJobByIndex = jobs[jobNum]
			}
		}
	}

	if runByID == nil && runByIndex == nil {
		ctx.NotFound(nil)
		return nil
	}

	if runByID != nil && runByIndex == nil {
		return runByID
	}

	if runByID == nil && runByIndex != nil {
		ctx.Redirect(fmt.Sprintf("%s/actions/runs/%d/jobs/%d", ctx.Repo.RepoLink, runByIndex.ID, targetJobByIndex.ID), http.StatusFound)
		return nil
	}

	// Reaching this point means both ID-based and legacy index-based interpretations are valid. Prefer the ID-based interpretation by default.
	// Use `by_index=1` query parameter to access the legacy index-based interpretation when necessary.
	return runByID
}

func getRunViewLink(run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt) string {
	if attempt == nil || run.LatestAttemptID == attempt.ID {
		return run.Link()
	}
	return fmt.Sprintf("%s/attempts/%d", run.Link(), attempt.Attempt)
}

// getCurrentRunJobsByPathParam resolves the current run view context from path parameters, including the run, optional attempt, and jobs to render.
// Any error will be written to the ctx, then the return values are all nil.
func getCurrentRunJobsByPathParam(ctx *context_module.Context) (*actions_model.ActionRun, *actions_model.ActionRunAttempt, []*actions_model.ActionRunJob) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return nil, nil, nil
	}
	run.Repo = ctx.Repo.Repository

	var err error
	var selectedJob *actions_model.ActionRunJob
	if ctx.PathParam("job") != "" {
		jobID := ctx.PathParamInt64("job")
		selectedJob, err = actions_model.GetRunJobByRunAndID(ctx, run.ID, jobID)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunJobByRepoAndID", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	}

	// Resolve the attempt to display.
	// Priority: explicit path param (/attempts/:num) > job's attempt (when navigating to a specific job) > latest attempt.
	// attempt may be nil for legacy runs that pre-date ActionRunAttempt; callers must handle that case.
	attemptNum := ctx.PathParamInt64("attempt")
	var attempt *actions_model.ActionRunAttempt
	switch {
	case attemptNum > 0:
		// Explicit attempt number in the URL — user is viewing a historical attempt.
		attempt, err = actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunAttemptByRunIDAndAttempt", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	case selectedJob != nil && selectedJob.RunAttemptID > 0:
		// No explicit attempt in the URL, but the requested job belongs to a known attempt — resolve via the job.
		attempt, err = actions_model.GetRunAttemptByRepoAndID(ctx, selectedJob.RepoID, selectedJob.RunAttemptID)
		if err != nil {
			ctx.NotFoundOrServerError("GetRunAttemptByRepoAndID", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	default:
		// No attempt context at all — show the latest attempt (nil for legacy runs).
		attempt, _, err = run.GetLatestAttempt(ctx)
		if err != nil {
			ctx.NotFoundOrServerError("GetLatestAttempt", func(err error) bool {
				return errors.Is(err, util.ErrNotExist)
			}, err)
			return nil, nil, nil
		}
	}

	// Resolve the jobs for the resolved attempt.
	// When attempt is nil (legacy run or legacy job), jobs are stored with run_attempt_id=0.
	var resolvedAttemptID int64
	if attempt != nil {
		resolvedAttemptID = attempt.ID
	}
	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, run.ID, resolvedAttemptID)
	if err != nil {
		ctx.ServerError("get current jobs", err)
		return nil, nil, nil
	}
	jobs.SortMatrixGroupsByName()

	for _, job := range jobs {
		job.Run = run
	}
	return run, attempt, jobs
}
