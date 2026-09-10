// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

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

// TestFindQueueJobs verifies the build-queue query: running jobs first, then the jobs a runner may still
// pick up (waiting + unclaimed + non-reusable) in pickup order.
func TestFindQueueJobs(t *testing.T) {
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
	jRunning := insert("running", StatusRunning, 998, false)
	// Rows that must be excluded from the queue.
	insert("claimed", StatusWaiting, 999, false) // already has a task
	insert("reusable", StatusWaiting, 0, true)   // reusable caller never runs on a runner

	// Force `updated` so pickup order among the queued jobs differs from insertion/id order: C < A < B.
	setUpdated := func(id, ts int64) {
		_, err := db.GetEngine(ctx).Exec("UPDATE `action_run_job` SET updated = ? WHERE id = ?", ts, id)
		require.NoError(t, err)
	}
	setUpdated(jC.ID, 100)
	setUpdated(jA.ID, 200)
	setUpdated(jB.ID, 300)

	ids := func(jobs []*ActionRunJob) []int64 {
		out := make([]int64, len(jobs))
		for i, j := range jobs {
			out[i] = j.ID
		}
		return out
	}

	jobs, total, err := FindQueueJobs(ctx, QueueJobsOptions{RepoID: repoID}, 1, 50)
	require.NoError(t, err)
	assert.EqualValues(t, 4, total, "one running job plus the waiting, unclaimed, non-reusable ones")
	assert.Equal(t, []int64{jRunning.ID, jC.ID, jA.ID, jB.ID}, ids(jobs), "running jobs head the list, the queued ones follow in pickup order")

	waiting, total, err := FindQueueJobs(ctx, QueueJobsOptions{RepoID: repoID, Status: StatusWaiting}, 1, 50)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	assert.Equal(t, []int64{jC.ID, jA.ID, jB.ID}, ids(waiting))

	running, total, err := FindQueueJobs(ctx, QueueJobsOptions{RepoID: repoID, Status: StatusRunning}, 1, 50)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, []int64{jRunning.ID}, ids(running))

	// One pager covers the whole list, so a page boundary can fall inside it.
	page2, _, err := FindQueueJobs(ctx, QueueJobsOptions{RepoID: repoID}, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, []int64{jB.ID}, ids(page2))

	filterRepoIDs, err := QueueFilterRepoIDs(ctx, QueueJobsOptions{RepoID: repoID}, 10)
	require.NoError(t, err)
	assert.Contains(t, filterRepoIDs, repoID, "a repo with pending work is offered by the filter")
}
