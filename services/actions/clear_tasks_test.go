// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createRunAttempt(t *testing.T, runIndex int64, concurrencyGroup string, status actions_model.Status) (*actions_model.ActionRun, *actions_model.ActionRunAttempt) {
	t.Helper()

	run := &actions_model.ActionRun{
		RepoID:        1,
		OwnerID:       2,
		TriggerUserID: 2,
		WorkflowID:    "test.yml",
		Index:         runIndex,
		Ref:           "refs/heads/main",
		Status:        status,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:           run.RepoID,
		RunID:            run.ID,
		Attempt:          1,
		TriggerUserID:    run.TriggerUserID,
		Status:           status,
		ConcurrencyGroup: concurrencyGroup,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))

	return run, attempt
}

func createConflictingCancellingJob(t *testing.T, concurrencyGroup string, runIndex int64) *actions_model.ActionRunJob {
	t.Helper()

	run, attempt := createRunAttempt(t, runIndex, concurrencyGroup, actions_model.StatusBlocked)
	job := &actions_model.ActionRunJob{
		RunID:            run.ID,
		RunAttemptID:     attempt.ID,
		AttemptJobID:     1,
		RepoID:           run.RepoID,
		OwnerID:          run.OwnerID,
		CommitSHA:        "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Name:             "conflicting-cancelling-job",
		JobID:            "conflicting-cancelling-job",
		Status:           actions_model.StatusCancelling,
		ConcurrencyGroup: concurrencyGroup,
	}
	require.NoError(t, db.Insert(t.Context(), job))

	return job
}

func TestCancellableJobs(t *testing.T) {
	jobs := []*actions_model.ActionRunJob{
		{ID: 1, JobID: "always", Status: actions_model.StatusRunning, WorkflowPayload: []byte(`jobs: {always: {if: "always() && needs.build.result == 'cancelled'"}}`)},
		{ID: 2, JobID: "ordinary", Status: actions_model.StatusBlocked, WorkflowPayload: []byte(`jobs: {ordinary: {}}`)},
		{ID: 3, JobID: "not-cancelled", Status: actions_model.StatusBlocked, WorkflowPayload: []byte(`jobs: {not-cancelled: {if: "always() && !cancelled()"}}`)},
		{ID: 4, JobID: "done", Status: actions_model.StatusSuccess, WorkflowPayload: []byte(`jobs: {done: {if: "${{ always() }}"}}`)},
	}
	for _, test := range []struct {
		name    string
		started timeutil.TimeStamp
		status  actions_model.Status
		force   bool
		want    []*actions_model.ActionRunJob
	}{
		{name: "pending run", want: jobs},
		{name: "started run", started: 1, want: jobs[1:]},
		{name: "legacy running run", status: actions_model.StatusRunning, want: jobs[1:]},
		{name: "force cancellation", started: 1, force: true, want: jobs},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, cancellableJobs(&actions_model.ActionRun{Started: test.started, Status: test.status}, jobs, test.force))
		})
	}
}

func TestShouldBlockJobByConcurrency_CancellingJobBlocks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const concurrencyGroup = "test-cancelling-job-blocks"
	createConflictingCancellingJob(t, concurrencyGroup, 9903)

	job := &actions_model.ActionRunJob{
		RepoID:                 1,
		RawConcurrency:         concurrencyGroup,
		IsConcurrencyEvaluated: true,
		ConcurrencyGroup:       concurrencyGroup,
	}

	shouldBlock, err := shouldBlockJobByConcurrency(t.Context(), job)
	require.NoError(t, err)
	assert.True(t, shouldBlock)
	job.ConcurrencyCancel = true
	shouldBlock, err = shouldBlockJobByConcurrency(t.Context(), job)
	require.NoError(t, err)
	assert.True(t, shouldBlock)
}

func TestShouldBlockJobByConcurrency_OwnAttemptDoesNotBlock(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const concurrencyGroup = "test-own-attempt-does-not-block"
	run, attempt := createRunAttempt(t, 9906, concurrencyGroup, actions_model.StatusWaiting)
	job := &actions_model.ActionRunJob{
		RunID:                  run.ID,
		RunAttemptID:           attempt.ID,
		RepoID:                 run.RepoID,
		RawConcurrency:         concurrencyGroup,
		IsConcurrencyEvaluated: true,
		ConcurrencyGroup:       concurrencyGroup,
	}
	shouldBlock, err := shouldBlockJobByConcurrency(t.Context(), job)
	require.NoError(t, err)
	assert.False(t, shouldBlock)

	createRunAttempt(t, 9907, concurrencyGroup, actions_model.StatusWaiting) // another run's attempt still holds the group
	shouldBlock, err = shouldBlockJobByConcurrency(t.Context(), job)
	require.NoError(t, err)
	assert.True(t, shouldBlock)
}

func TestPrepareToStartWithConcurrencyWaitingJob(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const concurrencyGroup = "test-waiting-job-keeps-concurrency-slot"
	run, attempt := createRunAttempt(t, 9905, "", actions_model.StatusWaiting)
	previousJob := &actions_model.ActionRunJob{
		RunID:            run.ID,
		RunAttemptID:     attempt.ID,
		AttemptJobID:     1,
		RepoID:           run.RepoID,
		OwnerID:          run.OwnerID,
		CommitSHA:        "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Name:             "waiting-job",
		JobID:            "waiting-job",
		Status:           actions_model.StatusWaiting,
		ConcurrencyGroup: concurrencyGroup,
	}
	require.NoError(t, db.Insert(t.Context(), previousJob))

	newJob := &actions_model.ActionRunJob{
		RepoID:                 run.RepoID,
		RawConcurrency:         concurrencyGroup,
		IsConcurrencyEvaluated: true,
		ConcurrencyGroup:       concurrencyGroup,
	}
	status, cancelled, err := PrepareToStartJobWithConcurrency(t.Context(), newJob)
	require.NoError(t, err)

	assert.Equal(t, actions_model.StatusBlocked, status)
	assert.Empty(t, cancelled)

	newAttempt := &actions_model.ActionRunAttempt{
		RepoID:           run.RepoID,
		ConcurrencyGroup: concurrencyGroup,
	}
	status, cancelled, err = PrepareToStartRunWithConcurrency(t.Context(), newAttempt)
	require.NoError(t, err)

	assert.Equal(t, actions_model.StatusBlocked, status)
	assert.Empty(t, cancelled)
	assert.Equal(t, actions_model.StatusWaiting, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: previousJob.ID}).Status)

	// a cancelled waiting job releases the group at once
	newJob.ConcurrencyCancel = true
	status, cancelled, err = PrepareToStartJobWithConcurrency(t.Context(), newJob)
	require.NoError(t, err)

	assert.Equal(t, actions_model.StatusWaiting, status)
	require.Len(t, cancelled, 1)
	assert.Equal(t, previousJob.ID, cancelled[0].ID)
}

func TestShouldBlockRunByConcurrency_CancellingJobBlocks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const concurrencyGroup = "test-cancelling-run-blocks"
	createConflictingCancellingJob(t, concurrencyGroup, 9904)

	attempt := &actions_model.ActionRunAttempt{
		RepoID:           1,
		ConcurrencyGroup: concurrencyGroup,
	}

	shouldBlock, err := shouldBlockRunByConcurrency(t.Context(), attempt)
	require.NoError(t, err)
	assert.True(t, shouldBlock)
	attempt.ConcurrencyCancel = true
	shouldBlock, err = shouldBlockRunByConcurrency(t.Context(), attempt)
	require.NoError(t, err)
	assert.True(t, shouldBlock)
}

// TestStopEndlessTasksSkipsCancelling verifies that a task running its post-cancel cleanup is not
// force-stopped by the endless-task sweep just because the job started long ago.
func TestStopEndlessTasksSkipsCancelling(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// StopEndlessTasks emits ready jobs onto the emitter queue, mock it
	defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(runID int64) error { return nil })()

	// well past the endless-task threshold, keyed on the task's start time
	longAgo := timeutil.TimeStamp(time.Now().Add(-2 * setting.Actions.EndlessTaskTimeout).Unix())

	var seq int64
	newTaskWithJob := func(status actions_model.Status) *actions_model.ActionTask {
		seq++
		run := &actions_model.ActionRun{
			RepoID: 1, OwnerID: 2, TriggerUserID: 2, WorkflowID: "test.yml",
			Index: 99500 + seq, Ref: "refs/heads/main", Status: actions_model.StatusRunning,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		attempt := &actions_model.ActionRunAttempt{
			RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: run.TriggerUserID, Status: actions_model.StatusRunning,
		}
		require.NoError(t, db.Insert(t.Context(), attempt))
		job := &actions_model.ActionRunJob{
			RunID: run.ID, RunAttemptID: attempt.ID, AttemptJobID: 1, RepoID: run.RepoID, OwnerID: run.OwnerID,
			CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Name: "j", JobID: "j", Status: status,
		}
		require.NoError(t, db.Insert(t.Context(), job))
		task := &actions_model.ActionTask{
			JobID: job.ID, RepoID: run.RepoID, OwnerID: run.OwnerID,
			CommitSHA: job.CommitSHA, Status: status, Started: longAgo,
			TokenHash: fmt.Sprintf("endless-test-token-%d", seq), TokenSalt: "salt",
		}
		require.NoError(t, db.Insert(t.Context(), task))
		return task
	}

	running := newTaskWithJob(actions_model.StatusRunning)
	cancelling := newTaskWithJob(actions_model.StatusCancelling)

	require.NoError(t, StopEndlessTasks(t.Context()))

	runningAfter := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: running.ID})
	cancellingAfter := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: cancelling.ID})
	assert.Equal(t, actions_model.StatusFailure, runningAfter.Status, "long-running task should be force-stopped")
	assert.Equal(t, actions_model.StatusCancelling, cancellingAfter.Status, "cancelling task should keep running its cleanup")
}
