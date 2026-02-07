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

// TestMaxParallelOne_SimpleSequence tests the core issue: max-parallel=1 execution
func TestMaxParallelOne_SimpleSequence(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	runID := int64(11000)
	jobID := "simple-sequence-job"
	maxParallel := 1

	// Create ActionRun
	run := &ActionRun{
		ID:      runID,
		RepoID:  1,
		OwnerID: 1,
		Index:   11000,
		Status:  StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, run))

	// Create jobs
	job1 := &ActionRunJob{
		RunID:       runID,
		RepoID:      1,
		OwnerID:     1,
		JobID:       jobID,
		Name:        "Job 1",
		Status:      StatusWaiting,
		MaxParallel: maxParallel,
	}
	job2 := &ActionRunJob{
		RunID:       runID,
		RepoID:      1,
		OwnerID:     1,
		JobID:       jobID,
		Name:        "Job 2",
		Status:      StatusWaiting,
		MaxParallel: maxParallel,
	}
	job3 := &ActionRunJob{
		RunID:       runID,
		RepoID:      1,
		OwnerID:     1,
		JobID:       jobID,
		Name:        "Job 3",
		Status:      StatusWaiting,
		MaxParallel: maxParallel,
	}

	assert.NoError(t, db.Insert(ctx, job1))
	assert.NoError(t, db.Insert(ctx, job2))
	assert.NoError(t, db.Insert(ctx, job3))

	// TEST 1: Initially, 0 running
	running, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 0, running, "Should have 0 jobs running initially")

	// TEST 2: Job1 starts
	job1.Status = StatusRunning
	_, err = UpdateRunJob(ctx, job1, nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 1, running, "Should have 1 job running after job1 starts")

	// TEST 3: Job1 completes
	job1.Status = StatusSuccess
	_, err = UpdateRunJob(ctx, job1, nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 0, running, "Should have 0 jobs running after job1 completes - THIS IS THE CRITICAL TEST")

	// TEST 4: Job2 should now be able to start
	job2.Status = StatusRunning
	_, err = UpdateRunJob(ctx, job2, nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 1, running, "Should have 1 job running after job2 starts - IF THIS FAILS, THE BUG IS NOT FIXED")

	// TEST 5: Job2 completes
	job2.Status = StatusSuccess
	_, err = UpdateRunJob(ctx, job2, nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 0, running, "Should have 0 jobs running after job2 completes")

	// TEST 6: Job3 starts
	job3.Status = StatusRunning
	_, err = UpdateRunJob(ctx, job3, nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 1, running, "Should have 1 job running after job3 starts")

	t.Log("✅ All sequential execution tests passed!")
}

// TestMaxParallelOne_FreshJobFetch tests the fresh job fetch mechanism
func TestMaxParallelOne_FreshJobFetch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	runID := int64(12000)
	jobID := "fresh-fetch-job"

	// Create ActionRun
	run := &ActionRun{
		ID:      runID,
		RepoID:  1,
		OwnerID: 1,
		Index:   12000,
		Status:  StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, run))

	// Create a job
	job := &ActionRunJob{
		RunID:   runID,
		RepoID:  1,
		OwnerID: 1,
		JobID:   jobID,
		Name:    "Fresh Fetch Test Job",
		Status:  StatusWaiting,
	}
	assert.NoError(t, db.Insert(ctx, job))

	// Fetch fresh copy (simulating CreateTaskForRunner behavior)
	freshJob, err := GetRunJobByID(ctx, job.ID)
	assert.NoError(t, err)
	assert.NotNil(t, freshJob)
	assert.Equal(t, StatusWaiting, freshJob.Status, "Fresh job should have WAITING status")
	assert.Equal(t, int64(0), freshJob.TaskID, "Fresh job should have TaskID=0")

	// Update original job to RUNNING
	job.Status = StatusRunning
	_, err = UpdateRunJob(ctx, job, nil)
	assert.NoError(t, err)

	// Fetch fresh copy again - should reflect the update
	freshJob2, err := GetRunJobByID(ctx, job.ID)
	assert.NoError(t, err)
	assert.NotNil(t, freshJob2)
	assert.Equal(t, StatusRunning, freshJob2.Status, "Fresh job should now have RUNNING status")

	t.Log("✅ Fresh job fetch mechanism works correctly!")
}

// TestCountRunningJobs tests the CountRunningJobsByWorkflowAndRun function
func TestCountRunningJobs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	runID := int64(13000)
	jobID := "count-jobs"

	// Create ActionRun
	run := &ActionRun{
		ID:      runID,
		RepoID:  1,
		OwnerID: 1,
		Index:   13000,
		Status:  StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, run))

	// Create 5 jobs
	for i := 1; i <= 5; i++ {
		job := &ActionRunJob{
			RunID:   runID,
			RepoID:  1,
			OwnerID: 1,
			JobID:   jobID,
			Name:    "Job " + string(rune(i)),
			Status:  StatusWaiting,
		}
		assert.NoError(t, db.Insert(ctx, job))
	}

	// Get all jobs
	jobs, err := GetRunJobsByRunID(ctx, runID)
	assert.NoError(t, err)
	assert.Len(t, jobs, 5)

	// Initially 0 running
	running, err := CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 0, running)

	// Set job 0 to RUNNING
	jobs[0].Status = StatusRunning
	_, err = UpdateRunJob(ctx, jobs[0], nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 1, running)

	// Set job 1 to RUNNING
	jobs[1].Status = StatusRunning
	_, err = UpdateRunJob(ctx, jobs[1], nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 2, running)

	// Set job 0 to SUCCESS (not counted as RUNNING anymore)
	jobs[0].Status = StatusSuccess
	_, err = UpdateRunJob(ctx, jobs[0], nil)
	assert.NoError(t, err)

	running, err = CountRunningJobsByWorkflowAndRun(ctx, runID, jobID)
	assert.NoError(t, err)
	assert.Equal(t, 1, running)

	t.Log("✅ Count running jobs works correctly!")
}
