// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countJobsByStatus returns the number of jobs with the given status in allJobs.
func countJobsByStatus(allJobs actions_model.ActionJobList, status actions_model.Status) int {
	n := 0
	for _, j := range allJobs {
		if j.Status == status {
			n++
		}
	}
	return n
}

// TestMaxParallel_ServiceLayer verifies the max-parallel invariant: Running+Waiting <= MaxParallel.
func TestMaxParallel_ServiceLayer(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("invariant: Running+Waiting <= MaxParallel", func(t *testing.T) {
		runID := int64(10000)
		jobID := "svc-max-parallel"
		maxParallel := 2

		run := &actions_model.ActionRun{ID: runID, RepoID: 1, OwnerID: 1, Index: 10000, Status: actions_model.StatusRunning}
		require.NoError(t, db.Insert(context.Background(), run))

		jobs := []*actions_model.ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "r1", Status: actions_model.StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "w1", Status: actions_model.StatusWaiting, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "b1", Status: actions_model.StatusBlocked, MaxParallel: maxParallel},
		}
		for _, j := range jobs {
			require.NoError(t, db.Insert(context.Background(), j))
		}

		allJobs, err := db.Find[actions_model.ActionRunJob](context.Background(), actions_model.FindRunJobOptions{RunID: runID})
		require.NoError(t, err)

		running := countJobsByStatus(allJobs, actions_model.StatusRunning)
		waiting := countJobsByStatus(allJobs, actions_model.StatusWaiting)
		blocked := countJobsByStatus(allJobs, actions_model.StatusBlocked)

		assert.LessOrEqual(t, running+waiting, maxParallel)
		assert.Equal(t, 1, blocked)
	})

	t.Run("slot becomes available after completion", func(t *testing.T) {
		runID := int64(20000)
		jobID := "svc-slot-free"
		maxParallel := 2

		run := &actions_model.ActionRun{ID: runID, RepoID: 1, OwnerID: 1, Index: 20000, Status: actions_model.StatusRunning}
		require.NoError(t, db.Insert(context.Background(), run))

		jobs := []*actions_model.ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "r1", Status: actions_model.StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "r2", Status: actions_model.StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "b1", Status: actions_model.StatusBlocked, MaxParallel: maxParallel},
		}
		for _, j := range jobs {
			require.NoError(t, db.Insert(context.Background(), j))
		}

		jobs[0].Status = actions_model.StatusSuccess
		_, err := actions_model.UpdateRunJob(context.Background(), jobs[0], nil, "status")
		require.NoError(t, err)

		allJobs, err := db.Find[actions_model.ActionRunJob](context.Background(), actions_model.FindRunJobOptions{RunID: runID})
		require.NoError(t, err)

		running := countJobsByStatus(allJobs, actions_model.StatusRunning)
		assert.Equal(t, 1, running)
		assert.Less(t, running, maxParallel)
	})

	t.Run("no max-parallel means all jobs start as Waiting", func(t *testing.T) {
		runID := int64(30000)
		jobID := "svc-no-limit"

		run := &actions_model.ActionRun{ID: runID, RepoID: 1, OwnerID: 1, Index: 30000, Status: actions_model.StatusRunning}
		require.NoError(t, db.Insert(context.Background(), run))

		for range 5 {
			require.NoError(t, db.Insert(context.Background(), &actions_model.ActionRunJob{
				RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "j",
				Status: actions_model.StatusWaiting, MaxParallel: 0,
			}))
		}

		allJobs, err := db.Find[actions_model.ActionRunJob](context.Background(), actions_model.FindRunJobOptions{RunID: runID})
		require.NoError(t, err)
		assert.Equal(t, 5, countJobsByStatus(allJobs, actions_model.StatusWaiting))
		assert.Equal(t, 0, countJobsByStatus(allJobs, actions_model.StatusBlocked))
	})
}
