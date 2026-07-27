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

func TestActionJobList_SortMatrixGroupsByName(t *testing.T) {
	mk := func(jobID, name string) *ActionRunJob {
		return &ActionRunJob{JobID: jobID, Name: name}
	}
	names := func(jobs ActionJobList) []string {
		out := make([]string, len(jobs))
		for i, j := range jobs {
			out[i] = j.Name
		}
		return out
	}

	t.Run("matrix group sorted naturally", func(t *testing.T) {
		jobs := ActionJobList{
			mk("build", "build"),
			mk("test", "test (10)"),
			mk("test", "test (2)"),
			mk("test", "test (1)"),
			mk("deploy", "deploy"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"build", "test (1)", "test (2)", "test (10)", "deploy"}, names(jobs))
	})

	t.Run("non-adjacent same JobID stays in input order", func(t *testing.T) {
		jobs := ActionJobList{
			mk("test", "test (10)"),
			mk("build", "build"),
			mk("test", "test (1)"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"test (10)", "build", "test (1)"}, names(jobs))
	})

	t.Run("groups stay in input order", func(t *testing.T) {
		jobs := ActionJobList{
			mk("z", "z"),
			mk("a", "a"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"z", "a"}, names(jobs))
	})

	t.Run("empty and singleton", func(t *testing.T) {
		ActionJobList(nil).SortMatrixGroupsByName()
		jobs := ActionJobList{mk("only", "only")}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"only"}, names(jobs))
	})
}

// TestFindRunJobOptions_Queue verifies the build-queue query mirrors the runner pickup predicate:
// waiting + unclaimed + non-reusable jobs, ordered by (updated ASC, id ASC).
func TestFindRunJobOptions_Queue(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// A repo id no fixture or other test uses, so the counts/order below are not polluted.
	const repoID int64 = 987654

	insert := func(name string, status Status, taskID int64, reusable bool) *ActionRunJob {
		job := &ActionRunJob{
			RepoID:           repoID,
			OwnerID:          1,
			Name:             name,
			JobID:            name,
			Status:           status,
			TaskID:           taskID,
			IsReusableCaller: reusable,
		}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}

	// Genuinely queued jobs: waiting, unclaimed (task_id=0), not reusable callers.
	jA := insert("a", StatusWaiting, 0, false)
	jB := insert("b", StatusWaiting, 0, false)
	jC := insert("c", StatusWaiting, 0, false)
	// Rows that must be excluded from the queue.
	insert("claimed", StatusWaiting, 999, false) // already has a task
	insert("reusable", StatusWaiting, 0, true)   // reusable caller never runs on a runner
	insert("running", StatusRunning, 998, false) // running, no longer queued

	// Force `updated` so pickup order (updated ASC, id ASC) differs from insertion/id order: C < A < B.
	setUpdated := func(id, ts int64) {
		_, err := db.GetEngine(ctx).Exec("UPDATE `action_run_job` SET updated = ? WHERE id = ?", ts, id)
		require.NoError(t, err)
	}
	setUpdated(jC.ID, 100)
	setUpdated(jA.ID, 200)
	setUpdated(jB.ID, 300)

	jobs, total, err := db.FindAndCount[ActionRunJob](ctx, FindRunJobOptions{
		RepoID:           repoID,
		Statuses:         []Status{StatusWaiting},
		IsReusableCaller: optional.Some(false),
		HasTask:          optional.Some(false),
		OrderBy:          QueuedJobsOrderBy,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 3, total, "only waiting, unclaimed, non-reusable jobs are queued")

	gotIDs := make([]int64, len(jobs))
	for i, j := range jobs {
		gotIDs[i] = j.ID
	}
	assert.Equal(t, []int64{jC.ID, jA.ID, jB.ID}, gotIDs, "queue is ordered by (updated ASC, id ASC)")
}
