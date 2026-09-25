// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

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

func TestFindQueueJobs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	const repoID int64 = 987654

	insert := func(status Status, taskID int64, reusable bool, updated timeutil.TimeStamp) int64 {
		job := &ActionRunJob{RepoID: repoID, Status: status, TaskID: taskID, IsReusableCaller: reusable, Updated: updated}
		_, err := db.GetEngine(ctx).NoAutoTime().Insert(job)
		require.NoError(t, err)
		return job.ID
	}
	queuedA := insert(StatusWaiting, 0, false, 200)
	queuedB := insert(StatusWaiting, 0, false, 300)
	queuedC := insert(StatusWaiting, 0, false, 100)
	running := insert(StatusRunning, 998, false, 0)
	cancelling := insert(StatusCancelling, 999, false, 0)
	insert(StatusWaiting, 999, false, 0)
	insert(StatusWaiting, 0, true, 0)

	find := func(status Status, page, pageSize int) (ids []int64, total int64) {
		jobs, total, err := FindQueueJobs(ctx, QueueJobsOptions{RepoID: repoID, Status: status}, page, pageSize)
		require.NoError(t, err)
		for _, job := range jobs {
			ids = append(ids, job.ID)
		}
		return ids, total
	}

	ids, total := find(StatusUnknown, 1, 10)
	assert.EqualValues(t, 5, total)
	assert.Equal(t, []int64{running, cancelling, queuedC, queuedA, queuedB}, ids)

	ids, _ = find(StatusWaiting, 1, 10)
	assert.Equal(t, []int64{queuedC, queuedA, queuedB}, ids)

	ids, _ = find(StatusRunning, 1, 10)
	assert.Equal(t, []int64{running, cancelling}, ids)

	ids, _ = find(StatusUnknown, 99, 3)
	assert.Equal(t, []int64{queuedA, queuedB}, ids)

	repoIDs, err := QueueFilterRepoIDs(ctx, 1000)
	require.NoError(t, err)
	assert.Contains(t, repoIDs, repoID)
}
