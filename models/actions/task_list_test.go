// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTasksWithCappedCount(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// fixtures for tasks and jobs are not loaded in this package, so create them with the large columns filled
	ctx := t.Context()
	run := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: 791})
	_, err := db.GetEngine(ctx).ID(run.ID).Cols("event_payload").Update(&ActionRun{EventPayload: "event"})
	require.NoError(t, err)
	job := &ActionRunJob{RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, Name: "job", JobID: "job", WorkflowPayload: []byte("payload")}
	_, err = db.GetEngine(ctx).Insert(job)
	require.NoError(t, err)

	const runnerID = 987654
	for i := range 5 {
		task := &ActionTask{JobID: job.ID, RunnerID: runnerID, RepoID: run.RepoID, OwnerID: run.OwnerID, TokenHash: "task-list-test-" + strconv.Itoa(i), LogIndexes: LogIndexes{1, 2, 3}}
		_, err = db.GetEngine(ctx).Insert(task)
		require.NoError(t, err)
	}

	t.Run("SkipLargeColumns", func(t *testing.T) {
		tasks, count, err := FindTasksWithCappedCount(ctx, FindTaskOptions{RunnerID: runnerID, ListOptions: db.ListOptions{Page: 1, PageSize: 30}}, 100)
		require.NoError(t, err)
		require.Equal(t, int64(5), count)
		require.Len(t, tasks, 5)
		require.NoError(t, TaskList(tasks).LoadAttributes(ctx))

		found := tasks[0]
		assert.Greater(t, found.ID, tasks[4].ID) // newest first
		assert.Empty(t, found.LogIndexes)
		require.NotNil(t, found.Job)
		assert.Empty(t, found.Job.WorkflowPayload)
		require.NotNil(t, found.Job.Run)
		assert.Equal(t, run.ID, found.Job.Run.ID)
		assert.Empty(t, found.Job.Run.EventPayload)
		require.NotNil(t, found.Job.Run.Repo)
		assert.NotEmpty(t, found.GetRunJobLink())
	})

	t.Run("CappedCount", func(t *testing.T) {
		tasks, count, err := FindTasksWithCappedCount(ctx, FindTaskOptions{RunnerID: runnerID, ListOptions: db.ListOptions{Page: 1, PageSize: 2}}, 3)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, int64(3), count)
	})

	t.Run("ExactCount", func(t *testing.T) {
		tasks, count, err := FindTasksWithCappedCount(ctx, FindTaskOptions{RunnerID: runnerID, ListOptions: db.ListOptions{Page: 2, PageSize: 2}}, 0)
		require.NoError(t, err)
		assert.Len(t, tasks, 2)
		assert.Equal(t, int64(5), count)
	})
}

func TestActionTaskRunnerIDIDIndex(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	tables, err := db.GetXORMEngineForTesting().DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name != "action_task" {
			continue
		}
		var cols [][]string
		for _, idx := range table.Indexes {
			cols = append(cols, idx.Cols)
		}
		assert.Contains(t, cols, []string{"runner_id", "id"})
		return
	}
	t.Fatal("action_task table not found")
}
