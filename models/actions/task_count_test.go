// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestCountRunningTasksByRunner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("NoRunningTasks", func(t *testing.T) {
		count, err := CountRunningTasksByRunner(ctx, 999999)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("WithRunningTasks", func(t *testing.T) {
		// Create a runner
		runner := &ActionRunner{
			UUID:      "test-runner-tasks",
			Name:      "Test Runner",
			OwnerID:   0,
			RepoID:    0,
			TokenHash: "test_hash_tasks",
			Token:     "test_token_tasks",
		}
		assert.NoError(t, db.Insert(ctx, runner))

		// Create running tasks
		task1 := &ActionTask{
			JobID:     1,
			RunnerID:  runner.ID,
			Status:    StatusRunning,
			RepoID:    1,
			OwnerID:   1,
			TokenHash: "task1_hash",
			Token:     "task1_token",
		}
		assert.NoError(t, db.Insert(ctx, task1))

		task2 := &ActionTask{
			JobID:     2,
			RunnerID:  runner.ID,
			Status:    StatusRunning,
			RepoID:    1,
			OwnerID:   1,
			TokenHash: "task2_hash",
			Token:     "task2_token",
		}
		assert.NoError(t, db.Insert(ctx, task2))

		// Count should be 2
		count, err := CountRunningTasksByRunner(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("MixedStatusTasks", func(t *testing.T) {
		runner := &ActionRunner{
			UUID:      "test-runner-mixed",
			Name:      "Mixed Status Runner",
			Capacity:  5,
			TokenHash: "mixed_runner_hash",
			Token:     "mixed_runner_token",
		}
		assert.NoError(t, db.Insert(ctx, runner))

		// Create tasks with different statuses
		statuses := []Status{StatusRunning, StatusSuccess, StatusRunning, StatusFailure, StatusWaiting}
		for i, status := range statuses {
			task := &ActionTask{
				JobID:     int64(100 + i),
				RunnerID:  runner.ID,
				Status:    status,
				RepoID:    1,
				OwnerID:   1,
				TokenHash: "mixed_task_hash_" + string(rune('a'+i)),
				Token:     "mixed_task_token_" + string(rune('a'+i)),
			}
			assert.NoError(t, db.Insert(ctx, task))
		}

		// Only 2 running tasks
		count, err := CountRunningTasksByRunner(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestCountRunningJobsByWorkflowAndRun(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("NoRunningJobs", func(t *testing.T) {
		count, err := CountRunningJobsByWorkflowAndRun(ctx, 999999, "nonexistent")
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("WithRunningJobs", func(t *testing.T) {
		runID := int64(1000)
		jobID := "test-job"

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   1000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create running jobs
		for range 3 {
			job := &ActionRunJob{
				RunID:   runID,
				RepoID:  1,
				OwnerID: 1,
				JobID:   jobID,
				Name:    "Test Job",
				Status:  StatusRunning,
			}
			assert.NoError(t, db.Insert(ctx, job))
		}

		count, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("DifferentJobIDs", func(t *testing.T) {
		runID := int64(2000)

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   2000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create jobs with different job IDs
		for i := range 5 {
			job := &ActionRunJob{
				RunID:   runID,
				RepoID:  1,
				OwnerID: 1,
				JobID:   "job-" + string(rune('A'+i)),
				Name:    "Test Job",
				Status:  StatusRunning,
			}
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Count for specific job ID should be 1
		count, err := CountRunningJobsByWorkflowAndRun(ctx, runID, "job-A")
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("MatrixJobsWithMaxParallel", func(t *testing.T) {
		runID := int64(3000)
		jobID := "matrix-job"
		maxParallel := 2

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   3000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create matrix jobs
		jobs := []*ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 1", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 2", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 3", Status: StatusWaiting, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 4", Status: StatusWaiting, MaxParallel: maxParallel},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Count running jobs - should be 2 (matching max-parallel)
		count, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
		assert.Equal(t, maxParallel, count, "Running jobs should equal max-parallel")
	})
}
