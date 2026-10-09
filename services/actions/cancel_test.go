// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForceCancelRun_WakesConcurrencyWaiters(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run := &actions_model.ActionRun{
		Title:         "all-jobs-done",
		RepoID:        4,
		Index:         9821,
		OwnerID:       1,
		WorkflowID:    "test.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        actions_model.StatusCancelling,
	}
	require.NoError(t, db.Insert(ctx, run))
	attempt := &actions_model.ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: 1, Status: actions_model.StatusCancelling}
	require.NoError(t, db.Insert(ctx, attempt))
	run.LatestAttemptID = attempt.ID
	require.NoError(t, actions_model.UpdateRun(ctx, run, "latest_attempt_id"))
	job := &actions_model.ActionRunJob{
		RunID:        run.ID,
		RunAttemptID: attempt.ID,
		RepoID:       run.RepoID,
		OwnerID:      run.OwnerID,
		CommitSHA:    run.CommitSHA,
		Name:         "job1",
		JobID:        "job1",
		Attempt:      1,
		Status:       actions_model.StatusCancelled,
		Stopped:      timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(ctx, job))

	var emitted []int64
	defer func(orig func(int64) error) { EmitJobsIfReadyByRun = orig }(EmitJobsIfReadyByRun)
	EmitJobsIfReadyByRun = func(runID int64) error {
		emitted = append(emitted, runID)
		return nil
	}

	_, err := ForceCancelRun(ctx, run, []*actions_model.ActionRunJob{job})
	require.NoError(t, err)
	assert.Equal(t, []int64{run.ID}, emitted)
}
