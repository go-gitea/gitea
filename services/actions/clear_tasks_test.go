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
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createConflictingCancellingJob(t *testing.T, concurrencyGroup string, runIndex int64) *actions_model.ActionRunJob {
	t.Helper()

	run := &actions_model.ActionRun{
		RepoID:        1,
		OwnerID:       2,
		TriggerUserID: 2,
		WorkflowID:    "test.yml",
		Index:         runIndex,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusBlocked,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:           run.RepoID,
		RunID:            run.ID,
		Attempt:          1,
		TriggerUserID:    run.TriggerUserID,
		Status:           actions_model.StatusBlocked,
		ConcurrencyGroup: concurrencyGroup,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))

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
}

// TestStopEndlessTasksSkipsCancelling verifies that a task running its post-cancel cleanup
// (StatusCancelling) is not force-stopped by the endless-task sweep just because the job started
// long ago; only a genuinely long-running (StatusRunning) task is stopped.
func TestStopEndlessTasksSkipsCancelling(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// StopEndlessTasks emits ready jobs onto the emitter queue, which is otherwise only set up by Init.
	if jobEmitterQueue == nil {
		jobEmitterQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "actions_ready_job_test", jobEmitterQueueHandler)
		require.NotNil(t, jobEmitterQueue)
	}

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
