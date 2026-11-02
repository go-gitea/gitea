// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForceCancelRun_SettledRunWakesConcurrencyWaiters(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// a run left cancelling although its caller and child are all cancelled already
	run := &actions_model.ActionRun{
		Title:         "settled-run",
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

	newJob := func(name string, parentID int64, isCaller bool) *actions_model.ActionRunJob {
		job := &actions_model.ActionRunJob{
			RunID:            run.ID,
			RunAttemptID:     attempt.ID,
			RepoID:           run.RepoID,
			OwnerID:          run.OwnerID,
			CommitSHA:        run.CommitSHA,
			Name:             name,
			JobID:            name,
			Attempt:          1,
			Status:           actions_model.StatusCancelled,
			Stopped:          timeutil.TimeStampNow(),
			ParentJobID:      parentID,
			IsReusableCaller: isCaller,
			IsExpanded:       isCaller,
		}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}
	caller := newJob("caller", 0, true)
	child := newJob("child", caller.ID, false)

	var emitted []int64
	defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(runID int64) error {
		emitted = append(emitted, runID)
		return nil
	})()

	got, err := ForceCancelRun(ctx, run, []*actions_model.ActionRunJob{caller, child})
	require.NoError(t, err)
	assert.Equal(t, actions_model.StatusCancelled, got.Status)
	assert.Equal(t, []int64{run.ID}, emitted)
}
