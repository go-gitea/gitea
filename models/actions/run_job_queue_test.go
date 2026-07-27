// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveQueuedJob(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// A repo id no fixture or other test uses, so ordering below is not polluted.
	const repoID int64 = 987655

	insert := func(name string, updated int64) *ActionRunJob {
		job := &ActionRunJob{
			RepoID:  repoID,
			OwnerID: 1,
			Name:    name,
			JobID:   name,
			Status:  StatusWaiting,
		}
		require.NoError(t, db.Insert(ctx, job))
		// Force `updated` so the natural FIFO order is deterministic and independent of insertion timing.
		_, err := db.GetEngine(ctx).Exec("UPDATE `action_run_job` SET updated = ? WHERE id = ?", updated, job.ID)
		require.NoError(t, err)
		return job
	}

	queueOrder := func() []string {
		jobs, err := db.Find[ActionRunJob](ctx, FindRunJobOptions{
			RepoID:           repoID,
			Statuses:         []Status{StatusWaiting},
			IsReusableCaller: optional.Some(false),
			HasTask:          optional.Some(false),
			OrderBy:          QueuedJobsOrderBy,
		})
		require.NoError(t, err)
		names := make([]string, len(jobs))
		for i, j := range jobs {
			names[i] = j.Name
		}
		return names
	}

	j1 := insert("j1", 100)
	j2 := insert("j2", 200)
	j3 := insert("j3", 300)
	j4 := insert("j4", 400)
	j5 := insert("j5", 500)

	// Default queue_rank is 0 for all, so the queue starts in pure FIFO order.
	assert.Equal(t, []string{"j1", "j2", "j3", "j4", "j5"}, queueOrder())

	// Promote j5 to the top (dropped above j1).
	ok, err := MoveQueuedJob(ctx, repoID, 0, 1, 50, j5.ID, 0, j1.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"j5", "j1", "j2", "j3", "j4"}, queueOrder())

	// Move j1 into the middle (between j3 and j4 in the current order).
	ok, err = MoveQueuedJob(ctx, repoID, 0, 1, 50, j1.ID, j3.ID, j4.ID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"j5", "j2", "j3", "j1", "j4"}, queueOrder())

	// Send j5 to the bottom (dropped below j4, no successor).
	ok, err = MoveQueuedJob(ctx, repoID, 0, 1, 50, j5.ID, j4.ID, 0)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"j2", "j3", "j1", "j4", "j5"}, queueOrder())

	// A newly queued job (rank 0) joins the tail and never jumps ahead of the curated order.
	insert("j6", 600)
	assert.Equal(t, []string{"j2", "j3", "j1", "j4", "j5", "j6"}, queueOrder())

	// Moving a job that has left the queue reports a stale view instead of erroring.
	_, err = db.GetEngine(ctx).Exec("UPDATE `action_run_job` SET status = ? WHERE id = ?", StatusRunning, j2.ID)
	require.NoError(t, err)
	ok, err = MoveQueuedJob(ctx, repoID, 0, 1, 50, j2.ID, 0, j3.ID)
	require.NoError(t, err)
	assert.False(t, ok, "moving a no-longer-queued job signals the caller to refresh")
}
