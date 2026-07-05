// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMatrixRun inserts a running run + attempt with `legs` running jobs, mimicking a
// fail-fast:false matrix, and returns the run and its jobs.
func newMatrixRun(ctx context.Context, t *testing.T, legs int) (*ActionRun, []*ActionRunJob) {
	t.Helper()

	index, err := db.GetNextResourceIndex(ctx, "action_run_index", 4)
	require.NoError(t, err)

	run := &ActionRun{
		Title:         "matrix-fail",
		RepoID:        4,
		Index:         index,
		OwnerID:       1,
		WorkflowID:    "pr.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "pull_request",
		TriggerEvent:  "pull_request",
		EventPayload:  "{}",
		Status:        StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))

	attempt := &ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: 1, Status: StatusRunning}
	require.NoError(t, db.Insert(ctx, attempt))
	run.LatestAttemptID = attempt.ID
	require.NoError(t, UpdateRun(ctx, run, "latest_attempt_id"))

	jobs := make([]*ActionRunJob, 0, legs)
	for range legs {
		job := &ActionRunJob{
			RunID: run.ID, RunAttemptID: attempt.ID, RepoID: run.RepoID, OwnerID: run.OwnerID,
			CommitSHA: run.CommitSHA, Name: "checks", Attempt: 1, JobID: "checks", Status: StatusRunning,
		}
		require.NoError(t, db.Insert(ctx, job))
		jobs = append(jobs, job)
	}
	return run, jobs
}

// TestUpdateRunJobAggregatesAllFailure drives every leg of a fail-fast:false matrix to failure
// through the runner's UpdateTask path and asserts the run aggregates to failure. Regression test
// for issue #38333, where an all-failure matrix left the run stuck in running.
func TestUpdateRunJobAggregatesAllFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run, jobs := newMatrixRun(ctx, t, 5)
	for _, job := range jobs {
		// Same partial-job shape UpdateTaskByState uses when a runner reports a terminal result.
		_, err := UpdateRunJob(ctx, &ActionRunJob{ID: job.ID, RepoID: job.RepoID, Status: StatusFailure}, nil, "status")
		require.NoError(t, err)
	}

	got, err := GetRunByRepoAndID(ctx, run.RepoID, run.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailure, got.Status)

	attempt, err := GetRunAttemptByRepoAndID(ctx, run.RepoID, run.LatestAttemptID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailure, attempt.Status)
}
