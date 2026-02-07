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

func TestActionRunJob_MaxParallel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("NoMaxParallel", func(t *testing.T) {
		job := &ActionRunJob{
			RunID:       1,
			RepoID:      1,
			OwnerID:     1,
			JobID:       "test-job-1",
			Name:        "Test Job",
			Status:      StatusWaiting,
			MaxParallel: 0, // No limit
		}
		assert.NoError(t, db.Insert(ctx, job))

		retrieved, err := GetRunJobByID(ctx, job.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0, retrieved.MaxParallel)
	})

	t.Run("WithMaxParallel", func(t *testing.T) {
		job := &ActionRunJob{
			RunID:       1,
			RepoID:      1,
			OwnerID:     1,
			JobID:       "test-job-2",
			Name:        "Matrix Job",
			Status:      StatusWaiting,
			MaxParallel: 3,
		}
		assert.NoError(t, db.Insert(ctx, job))

		retrieved, err := GetRunJobByID(ctx, job.ID)
		assert.NoError(t, err)
		assert.Equal(t, 3, retrieved.MaxParallel)
	})

	t.Run("UpdateMaxParallel", func(t *testing.T) {
		// Create ActionRun first
		run := &ActionRun{
			ID:      1,
			RepoID:  1,
			OwnerID: 1,
			Status:  StatusRunning,
		}
		// Note: This might fail if run already exists from previous tests, but that's okay
		_ = db.Insert(ctx, run)

		job := &ActionRunJob{
			RunID:       1,
			RepoID:      1,
			OwnerID:     1,
			JobID:       "test-job-4",
			Name:        "Updatable Job",
			Status:      StatusWaiting,
			MaxParallel: 5,
		}
		assert.NoError(t, db.Insert(ctx, job))

		// Update max parallel
		job.MaxParallel = 10
		_, err := UpdateRunJob(ctx, job, nil, "max_parallel")
		assert.NoError(t, err)

		retrieved, err := GetRunJobByID(ctx, job.ID)
		assert.NoError(t, err)
		assert.Equal(t, 10, retrieved.MaxParallel)
	})
}

func TestActionRunJob_MaxParallelEnforcement(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("EnforceMaxParallel", func(t *testing.T) {
		runID := int64(5000)
		jobID := "parallel-enforced-job"
		maxParallel := 2

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   5000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create jobs simulating matrix execution
		jobs := []*ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 1", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 2", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 3", Status: StatusWaiting, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 4", Status: StatusWaiting, MaxParallel: maxParallel},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Verify running count
		runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, maxParallel, runningCount, "Should have exactly max-parallel jobs running")

		// Simulate job completion
		jobs[0].Status = StatusSuccess
		_, err = UpdateRunJob(ctx, jobs[0], nil, "status")
		assert.NoError(t, err)

		// Now running count should be 1
		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount)

		// Simulate next job starting
		jobs[2].Status = StatusRunning
		_, err = UpdateRunJob(ctx, jobs[2], nil, "status")
		assert.NoError(t, err)

		// Back to max-parallel
		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, maxParallel, runningCount)
	})

	t.Run("MaxParallelOne_SequentialExecution", func(t *testing.T) {
		runID := int64(6000)
		jobID := "sequential-job"
		maxParallel := 1

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   6000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create jobs simulating sequential execution with max-parallel=1
		jobs := []*ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 1", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 2", Status: StatusWaiting, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 3", Status: StatusWaiting, MaxParallel: maxParallel},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// Verify initial running count is 1
		runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running with max-parallel=1")

		// Complete first job
		jobs[0].Status = StatusSuccess
		_, err = UpdateRunJob(ctx, jobs[0], nil, "status")
		assert.NoError(t, err)

		// Now running count should be 0
		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 0, runningCount, "Should have 0 jobs running after first job completes")

		// Second job can now start
		jobs[1].Status = StatusRunning
		_, err = UpdateRunJob(ctx, jobs[1], nil, "status")
		assert.NoError(t, err)

		// Running count should be 1 again
		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running after second job starts")

		// Complete second job
		jobs[1].Status = StatusSuccess
		_, err = UpdateRunJob(ctx, jobs[1], nil, "status")
		assert.NoError(t, err)

		// Third job can now start
		jobs[2].Status = StatusRunning
		_, err = UpdateRunJob(ctx, jobs[2], nil, "status")
		assert.NoError(t, err)

		// Running count should still be 1
		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running after third job starts")
	})

	t.Run("MaxParallelOne_WithFailure", func(t *testing.T) {
		runID := int64(7000)
		jobID := "sequential-with-failure-job"
		maxParallel := 1

		// Create ActionRun first
		run := &ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   7000,
			Status:  StatusRunning,
		}
		assert.NoError(t, db.Insert(ctx, run))

		// Create jobs
		jobs := []*ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 1", Status: StatusRunning, MaxParallel: maxParallel},
			{RunID: runID, RepoID: 1, OwnerID: 1, JobID: jobID, Name: "Job 2", Status: StatusWaiting, MaxParallel: maxParallel},
		}

		for _, job := range jobs {
			assert.NoError(t, db.Insert(ctx, job))
		}

		// First job fails
		jobs[0].Status = StatusFailure
		_, err := UpdateRunJob(ctx, jobs[0], nil, "status")
		assert.NoError(t, err)

		// Verify no jobs are running
		runningCount, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 0, runningCount, "Should have 0 jobs running after job fails")

		// Second job can still start
		jobs[1].Status = StatusRunning
		_, err = UpdateRunJob(ctx, jobs[1], nil, "status")
		assert.NoError(t, err)

		runningCount, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 1, runningCount, "Should have exactly 1 job running after job starts")
	})
}
