// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

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
