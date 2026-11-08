// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertReemitRunAttempt(t *testing.T, run *actions_model.ActionRun, attemptNo int64, status actions_model.Status) *actions_model.ActionRunAttempt {
	t.Helper()

	attempt := &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       attemptNo,
		TriggerUserID: run.TriggerUserID,
		Status:        status,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))
	return attempt
}

func insertReemitRun(t *testing.T, runIndex int64, status actions_model.Status, needApproval bool) (*actions_model.ActionRun, *actions_model.ActionRunAttempt) {
	t.Helper()

	run := &actions_model.ActionRun{
		RepoID:        1,
		OwnerID:       2,
		TriggerUserID: 2,
		WorkflowID:    "test.yml",
		Index:         runIndex,
		Ref:           "refs/heads/main",
		Status:        status,
		NeedApproval:  needApproval,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	attempt := insertReemitRunAttempt(t, run, 1, status)
	_, err := db.Exec(t.Context(), "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", attempt.ID, run.ID)
	require.NoError(t, err)
	run.LatestAttemptID = attempt.ID
	return run, attempt
}

func insertReemitJob(t *testing.T, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, attemptJobID int64, jobID string, status actions_model.Status, needs ...string) *actions_model.ActionRunJob {
	t.Helper()

	job := &actions_model.ActionRunJob{
		RunID:           run.ID,
		RunAttemptID:    attempt.ID,
		AttemptJobID:    attemptJobID,
		RepoID:          run.RepoID,
		OwnerID:         run.OwnerID,
		JobID:           jobID,
		Name:            jobID,
		Status:          status,
		Needs:           needs,
		WorkflowPayload: minimalWorkflowPayload(jobID),
	}
	require.NoError(t, db.Insert(t.Context(), job))
	return job
}

// Test_ReEmitUnresolvedRuns verifies that the startup/periodic reconciliation
// only re-emits runs whose latest attempt lost a job-completion transition:
// a finished job exists, but a dependent job is still unresolved.
// https://github.com/go-gitea/gitea/issues/40279
func Test_ReEmitUnresolvedRuns(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	var emitted []int64
	defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(runID int64) error {
		emitted = append(emitted, runID)
		return nil
	})()

	// Stalled: "build" finished, "deploy" still blocked on it. This is the only
	// run the sweep must re-emit.
	stalledRun, stalledAttempt := insertReemitRun(t, 99201, actions_model.StatusBlocked, false)
	insertReemitJob(t, stalledRun, stalledAttempt, 1, "build", actions_model.StatusSuccess)
	insertReemitJob(t, stalledRun, stalledAttempt, 2, "deploy", actions_model.StatusBlocked, "build")

	// Healthy in-progress run: nothing finished yet, so nothing could have been lost.
	runningRun, runningAttempt := insertReemitRun(t, 99202, actions_model.StatusRunning, false)
	insertReemitJob(t, runningRun, runningAttempt, 1, "build", actions_model.StatusRunning)
	insertReemitJob(t, runningRun, runningAttempt, 2, "deploy", actions_model.StatusPending, "build")

	// Finished run: no unresolved jobs at all.
	doneRun, doneAttempt := insertReemitRun(t, 99203, actions_model.StatusSuccess, false)
	insertReemitJob(t, doneRun, doneAttempt, 1, "build", actions_model.StatusSuccess)
	insertReemitJob(t, doneRun, doneAttempt, 2, "deploy", actions_model.StatusSuccess, "build")

	// Awaiting approval: only an approval may release it, not the sweep.
	approvalRun, approvalAttempt := insertReemitRun(t, 99204, actions_model.StatusBlocked, true)
	insertReemitJob(t, approvalRun, approvalAttempt, 1, "build", actions_model.StatusSuccess)
	insertReemitJob(t, approvalRun, approvalAttempt, 2, "deploy", actions_model.StatusBlocked, "build")

	// Fresh run: jobs waiting for a runner, nothing finished yet.
	freshRun, freshAttempt := insertReemitRun(t, 99205, actions_model.StatusWaiting, false)
	insertReemitJob(t, freshRun, freshAttempt, 1, "build", actions_model.StatusWaiting)

	// Superseded attempt: the stale attempt looks stalled, but the latest
	// attempt is fully done, so the run must not be re-emitted.
	staleRun, staleAttempt1 := insertReemitRun(t, 99206, actions_model.StatusSuccess, false)
	insertReemitJob(t, staleRun, staleAttempt1, 1, "build", actions_model.StatusSuccess)
	insertReemitJob(t, staleRun, staleAttempt1, 2, "deploy", actions_model.StatusBlocked, "build")
	staleAttempt2 := insertReemitRunAttempt(t, staleRun, 2, actions_model.StatusSuccess)
	insertReemitJob(t, staleRun, staleAttempt2, 1, "build", actions_model.StatusSuccess)
	insertReemitJob(t, staleRun, staleAttempt2, 2, "deploy", actions_model.StatusSuccess, "build")
	_, err := db.Exec(ctx, "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", staleAttempt2.ID, staleRun.ID)
	require.NoError(t, err)

	require.NoError(t, ReEmitUnresolvedRuns(ctx))
	// Fixture run 793 also matches the stalled shape (a finished job with a
	// dependent still waiting), so it is legitimately re-emitted too. Assert
	// on the runs this test inserted instead of the exact set.
	assert.Contains(t, emitted, stalledRun.ID)
	for _, notEmitted := range []int64{runningRun.ID, doneRun.ID, approvalRun.ID, freshRun.ID, staleRun.ID} {
		assert.NotContains(t, emitted, notEmitted)
	}
}

// Test_checkJobsByRunID_UnblocksStalledDependent proves the repair the sweep
// triggers: re-running the per-run check on a stalled run moves its dependent
// job from Blocked to Waiting, so a runner can pick it up again.
// https://github.com/go-gitea/gitea/issues/40279
func Test_checkJobsByRunID_UnblocksStalledDependent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run, attempt := insertReemitRun(t, 99211, actions_model.StatusBlocked, false)
	insertReemitJob(t, run, attempt, 1, "build", actions_model.StatusSuccess)
	dependent := insertReemitJob(t, run, attempt, 2, "deploy", actions_model.StatusBlocked, "build")

	require.NoError(t, checkJobsByRunID(ctx, run.ID))

	refreshed := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: dependent.ID})
	assert.Equal(t, actions_model.StatusWaiting, refreshed.Status)
}
