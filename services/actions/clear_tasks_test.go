// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

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

func TestCancelAbandonedJobsCancelsWholeAttempt(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	prevQueue := jobEmitterQueue
	t.Cleanup(func() { jobEmitterQueue = prevQueue })
	jobEmitterQueue = queue.CreateUniqueQueue[*jobUpdate](t.Context(), "test-actions-ready-job", func(items ...*jobUpdate) []*jobUpdate { return nil })

	oldTS := timeutil.TimeStampNow().AddDuration(-setting.Actions.AbandonedJobTimeout - time.Second)

	run := &actions_model.ActionRun{
		RepoID:        1,
		OwnerID:       2,
		TriggerUserID: 2,
		WorkflowID:    "test.yml",
		Index:         9905,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       1,
		TriggerUserID: run.TriggerUserID,
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))
	_, err := db.GetEngine(t.Context()).Exec("UPDATE action_run SET latest_attempt_id = ? WHERE id = ?", attempt.ID, run.ID)
	require.NoError(t, err)

	runner := &actions_model.ActionRunner{
		UUID:                 "runner-abandoned-whole-attempt",
		Name:                 "runner-abandoned-whole-attempt",
		HasCancellingSupport: false,
	}
	require.NoError(t, db.Insert(t.Context(), runner))

	runningJob := &actions_model.ActionRunJob{
		RunID:        run.ID,
		RunAttemptID: attempt.ID,
		AttemptJobID: 1,
		RepoID:       run.RepoID,
		OwnerID:      run.OwnerID,
		CommitSHA:    "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Name:         "running-job",
		JobID:        "running-job",
		Status:       actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), runningJob))

	task := &actions_model.ActionTask{
		JobID:     runningJob.ID,
		Attempt:   1,
		RunnerID:  runner.ID,
		Status:    actions_model.StatusRunning,
		Started:   oldTS,
		RepoID:    run.RepoID,
		OwnerID:   run.OwnerID,
		CommitSHA: run.CommitSHA,
	}
	require.NoError(t, db.Insert(t.Context(), task))
	runningJob.TaskID = task.ID
	_, err = actions_model.UpdateRunJob(t.Context(), runningJob, nil, "task_id")
	require.NoError(t, err)

	abandonedJob := &actions_model.ActionRunJob{
		RunID:        run.ID,
		RunAttemptID: attempt.ID,
		AttemptJobID: 2,
		RepoID:       run.RepoID,
		OwnerID:      run.OwnerID,
		CommitSHA:    "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Name:         "abandoned-job",
		JobID:        "abandoned-job",
		Status:       actions_model.StatusWaiting,
		Created:      oldTS,
		Updated:      oldTS,
	}
	_, err = db.GetEngine(t.Context()).NoAutoTime().Insert(abandonedJob)
	require.NoError(t, err)

	require.NoError(t, CancelAbandonedJobs(t.Context()))

	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID}).Status)
	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: runningJob.ID}).Status)
	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: abandonedJob.ID}).Status)
	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: attempt.ID}).Status)
	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID}).Status)
}
