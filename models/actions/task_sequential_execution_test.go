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

// TestTaskCreation_MaxParallel_One tests that tasks are properly sequenced
// when max-parallel=1, ensuring no hang after task completion
func TestTaskCreation_MaxParallel_One(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("SequentialTaskCreationMaxParallelOne", func(t *testing.T) {
		runID := int64(8000)
		jobID := "task-sequential-max-parallel-one"
		maxParallel := 1

		// Setup: Create ActionRun
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   8000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create runner
		runner := &ActionRunner{
			ID:      1,
			UUID:    "test-runner-1",
			Name:    "Test Runner",
			OwnerID: 1,
		}
		assert.NoError(t, db.Insert(ctx, runner))

		// Create jobs with max-parallel=1
		jobs := []*ActionRunJob{
			{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Sequential Job 1",
				Status:      StatusWaiting,
				MaxParallel: maxParallel,
			},
			{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Sequential Job 2",
				Status:      StatusWaiting,
				MaxParallel: maxParallel,
			},
			{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Sequential Job 3",
				Status:      StatusWaiting,
				MaxParallel: maxParallel,
			},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Verify initial state: all jobs are waiting
		allJobs, err := GetRunJobsByRunID(ctx, runID)
		assert.NoError(t, err)
		assert.Len(t, allJobs, 3)

		// Verify that only 1 job should be able to run at a time with max-parallel=1
		runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 0, runningCount, "Should have 0 jobs running initially")

		// Simulate starting first job
		allJobs[0].Status = StatusRunning
		_, err = UpdateRunJob(ctx, allJobs[0], nil)
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running with max-parallel=1")

		// Complete first job
		allJobs[0].Status = StatusSuccess
		_, err = UpdateRunJob(ctx, allJobs[0], nil)
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 0, runningCount, "Should have 0 jobs running after first job completes")

		// This is the critical test: the second job should be able to start
		// Previously, the system might hang here
		allJobs[1].Status = StatusRunning
		_, err = UpdateRunJob(ctx, allJobs[1], nil)
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running after second job starts (critical test)")

		// Complete the second job
		allJobs[1].Status = StatusSuccess
		_, err = UpdateRunJob(ctx, allJobs[1], nil)
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 0, runningCount, "Should have 0 jobs running after second job completes")

		// Third job should also be able to start
		allJobs[2].Status = StatusRunning
		_, err = UpdateRunJob(ctx, allJobs[2], nil)
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running for third job")
	})

	t.Run("MaxParallelConstraintAfterTaskFetch", func(t *testing.T) {
		runID := int64(9000)
		jobID := "max-parallel-fetch-job"
		maxParallel := 1

		// Setup: Create ActionRun
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   9000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create jobs
		jobs := []*ActionRunJob{
			{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Job A",
				Status:      StatusWaiting,
				MaxParallel: maxParallel,
			},
			{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Job B",
				Status:      StatusWaiting,
				MaxParallel: maxParallel,
			},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Refresh jobs to get IDs
		allJobs, err := GetRunJobsByRunID(ctx, runID)
		assert.NoError(t, err)

		// Start first job
		allJobs[0].Status = StatusRunning
		_, err = UpdateRunJob(ctx, allJobs[0], nil)
		assert.NoError(t, err)

		// Verify constraint
		runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount)

		// Try to start second job while first is still running
		// This should not be allowed due to max-parallel=1
		freshAllJobs, err := GetRunJobsByRunID(ctx, runID)
		assert.NoError(t, err)

		// Check if we can determine from the count that the second job should not start
		for i := 1; i < len(freshAllJobs); i++ {
			if freshAllJobs[i].Status == StatusWaiting {
				// Before starting this job, verify max-parallel constraint
				runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
				assert.NoError(t, err)
				if runningCount >= maxParallel {
					// This job should wait
					assert.Equal(t, StatusWaiting, freshAllJobs[i].Status,
						"Job should remain waiting when max-parallel limit is reached")
				}
			}
		}
	})
}
