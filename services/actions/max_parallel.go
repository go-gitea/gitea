// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"math"
	"strconv"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/log"
)

// parseMaxParallel returns strategy.max-parallel for a job, 0 meaning unlimited.
// GitHub accepts any YAML number here and casts it to an int, so 1.5 truncates to 1.
// Expressions are not evaluated yet and fall back to unlimited.
func parseMaxParallel(jobID, maxParallelString string) int {
	if maxParallelString == "" {
		return 0
	}
	maxParallel, err := strconv.ParseFloat(maxParallelString, 64)
	if err != nil || math.IsNaN(maxParallel) {
		// Both fall back to unlimited, but an expression is a gap in Gitea while a non-number is the
		// author's mistake, so dropping the cap must not be reported the same way.
		if strings.Contains(maxParallelString, "${{") {
			// TODO: evaluate it against the contexts `if:` and the matrix already resolve, so that an
			// expression can actually cap a job instead of quietly disabling the cap.
			log.Debug("job %s: max-parallel %q is an expression, which is not evaluated yet: treating as unlimited", jobID, maxParallelString)
		} else {
			log.Warn("job %s: max-parallel %q is not a number, treating as unlimited", jobID, maxParallelString)
		}
		return 0
	}
	// a run can never hold more jobs than MaxJobNumPerRun, so clamping there keeps the cast total
	return int(min(max(maxParallel, 0), actions_model.MaxJobNumPerRun))
}

// maxParallelSlots counts the jobs holding a max-parallel slot. Slots are scoped by ParentJobID as
// well as JobID so the same job name in two reusable workflow calls does not cross-link, matching
// how jobStatusResolver scopes needs.
type maxParallelSlots map[maxParallelKey]int

type maxParallelKey struct {
	parentJobID int64
	jobID       string
}

// hold seeds a job's slot from a status it already holds, bypassing the limit check.
// The status is passed separately because the job emitter tracks statuses outside the job model.
func (s maxParallelSlots) hold(job *actions_model.ActionRunJob, status actions_model.Status) {
	if job.MaxParallel <= 0 {
		return // an unlimited job's count is never read by take, don't accumulate it
	}
	// A Cancelling job still owns its runner, so it keeps its slot until cleanup finishes.
	// An expanded reusable caller is underway even while its children aggregate it back to Blocked, so it holds a slot too.
	if status.In(actions_model.StatusRunning, actions_model.StatusWaiting, actions_model.StatusCancelling) ||
		(job.IsReusableCaller && job.IsExpanded && !status.IsDone()) {
		s[maxParallelKey{job.ParentJobID, job.JobID}]++
	}
}

// available reports whether a slot is free without reserving it.
// Peek it before running the concurrency check: DO NOT cancel the group's other pending jobs if no available slots.
func (s maxParallelSlots) available(job *actions_model.ActionRunJob) bool {
	return job.MaxParallel <= 0 || s[maxParallelKey{job.ParentJobID, job.JobID}] < job.MaxParallel
}

// take reserves a slot and reports whether one was free. A job without a limit always succeeds.
func (s maxParallelSlots) take(job *actions_model.ActionRunJob) bool {
	if !s.available(job) {
		return false
	}
	if job.MaxParallel > 0 {
		s[maxParallelKey{job.ParentJobID, job.JobID}]++
	}
	return true
}

// applyMaxParallel demotes a ready job to Blocked when its slots are full, leaving the job emitter to
// promote it once one frees. Call it after concurrency evaluation so a concurrency-blocked job does
// not consume a slot, and before the insert so a held-back reusable caller is not expanded.
func applyMaxParallel(job *actions_model.ActionRunJob, slots maxParallelSlots) {
	if job.Status == actions_model.StatusWaiting && !slots.take(job) {
		job.Status = actions_model.StatusBlocked
	}
}
