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

func getRunJobByID(ctx context.Context, t *testing.T, id int64) *ActionRunJob {
	t.Helper()
	got, exist, err := db.GetByID[ActionRunJob](ctx, id)
	require.NoError(t, err)
	require.True(t, exist)
	return got
}

// TestMaxParallel_FieldPersistence verifies that MaxParallel is stored and retrieved correctly.
func TestMaxParallel_FieldPersistence(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	run := &ActionRun{ID: 100, RepoID: 1, OwnerID: 1, Index: 100, Status: StatusRunning}
	require.NoError(t, db.Insert(ctx, run))

	t.Run("zero value means unlimited", func(t *testing.T) {
		job := &ActionRunJob{RunID: 100, RepoID: 1, OwnerID: 1, JobID: "no-limit", Name: "No Limit", Status: StatusWaiting, MaxParallel: 0}
		require.NoError(t, db.Insert(ctx, job))
		got := getRunJobByID(ctx, t, job.ID)
		assert.Equal(t, 0, got.MaxParallel)
	})

	t.Run("positive value is persisted", func(t *testing.T) {
		job := &ActionRunJob{RunID: 100, RepoID: 1, OwnerID: 1, JobID: "with-limit", Name: "With Limit", Status: StatusWaiting, MaxParallel: 3}
		require.NoError(t, db.Insert(ctx, job))
		got := getRunJobByID(ctx, t, job.ID)
		assert.Equal(t, 3, got.MaxParallel)
	})

	t.Run("can be updated via UpdateRunJob", func(t *testing.T) {
		job := &ActionRunJob{RunID: 100, RepoID: 1, OwnerID: 1, JobID: "updatable", Name: "Updatable", Status: StatusWaiting, MaxParallel: 5}
		require.NoError(t, db.Insert(ctx, job))
		job.MaxParallel = 10
		_, err := UpdateRunJob(ctx, job, nil, "max_parallel")
		require.NoError(t, err)
		got := getRunJobByID(ctx, t, job.ID)
		assert.Equal(t, 10, got.MaxParallel)
	})
}
