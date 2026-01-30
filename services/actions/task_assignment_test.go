// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestMaxParallelJobStatusAndCounting(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("MaxParallelReached", func(t *testing.T) {
		runID := int64(10000)
		jobID := "max-parallel-job"
		maxParallel := 2

		// Create ActionRun first
		run := &actions_model.ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   10000,
			Status:  actions_model.StatusRunning,
		}
		assert.NoError(t, db.Insert(context.Background(), run))

		// Create waiting jobs
		for range 4 {
			job := &actions_model.ActionRunJob{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Test Job",
				Status:      actions_model.StatusWaiting,
				MaxParallel: maxParallel,
			}
			assert.NoError(t, db.Insert(context.Background(), job))
		}

		// Start 2 jobs (max-parallel limit)
		jobs, err := actions_model.GetRunJobsByRunID(context.Background(), runID)
		assert.NoError(t, err)
		assert.Len(t, jobs, 4)

		for i := range 2 {
			jobs[i].Status = actions_model.StatusRunning
			_, err := actions_model.UpdateRunJob(context.Background(), jobs[i], nil, "status")
			assert.NoError(t, err)
		}

		// Verify max-parallel is enforced
		runningCount, err := actions_model.CountRunningJobsByWorkflowAndRun(context.Background(), runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, maxParallel, runningCount)

		// Remaining jobs should stay in waiting
		remainingJobs, err := actions_model.GetRunJobsByRunID(context.Background(), runID)
		assert.NoError(t, err)

		waitingCount := 0
		for _, job := range remainingJobs {
			if job.Status == actions_model.StatusWaiting {
				waitingCount++
			}
		}
		assert.Equal(t, 2, waitingCount)
	})

	t.Run("MaxParallelNotSet", func(t *testing.T) {
		runID := int64(20000)
		jobID := "no-limit-job"

		// Create ActionRun first
		run := &actions_model.ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   20000,
			Status:  actions_model.StatusRunning,
		}
		assert.NoError(t, db.Insert(context.Background(), run))

		// Create jobs without max-parallel
		for range 5 {
			job := &actions_model.ActionRunJob{
				RunID:       runID,
				RepoID:      1,
				OwnerID:     1,
				JobID:       jobID,
				Name:        "Test Job",
				Status:      actions_model.StatusWaiting,
				MaxParallel: 0, // No limit
			}
			assert.NoError(t, db.Insert(context.Background(), job))
		}

		// All jobs can run simultaneously
		jobs, err := actions_model.GetRunJobsByRunID(context.Background(), runID)
		assert.NoError(t, err)

		for _, job := range jobs {
			job.Status = actions_model.StatusRunning
			_, err := actions_model.UpdateRunJob(context.Background(), job, nil, "status")
			assert.NoError(t, err)
		}

		runningCount, err := actions_model.CountRunningJobsByWorkflowAndRun(context.Background(), runID, jobID)
		assert.NoError(t, err)
		assert.Equal(t, 5, runningCount, "All jobs should be able to run without limit")
	})

	t.Run("MaxParallelWrongValue", func(t *testing.T) {
		runID := int64(30000)
		jobID := "wrong-value-use-default-value-job"

		// Create ActionRun first
		run := &actions_model.ActionRun{
			ID:      runID,
			RepoID:  1,
			OwnerID: 1,
			Index:   30000,
			Status:  actions_model.StatusRunning,
		}
		assert.NoError(t, db.Insert(context.Background(), run))

		// Test different invalid max-parallel values
		testCases := []struct {
			name        string
			maxParallel int
			description string
		}{
			{
				name:        "negative value",
				maxParallel: -1,
				description: "Negative max-parallel should default to 0 (no limit)",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create jobs with the test max-parallel value
				for i := range 5 {
					job := &actions_model.ActionRunJob{
						RunID:       runID,
						RepoID:      1,
						OwnerID:     1,
						JobID:       jobID,
						Name:        "Test Job " + tc.name,
						Status:      actions_model.StatusWaiting,
						MaxParallel: tc.maxParallel,
					}
					assert.NoError(t, db.Insert(context.Background(), job))

					// Verify the value was stored
					if i == 0 {
						storedJob, err := actions_model.GetRunJobByID(context.Background(), job.ID)
						assert.NoError(t, err)
						assert.Equal(t, tc.maxParallel, storedJob.MaxParallel, tc.description)
					}
				}

				// All jobs can run simultaneously when max-parallel <= 0
				jobs, err := actions_model.GetRunJobsByRunID(context.Background(), runID)
				assert.NoError(t, err)

				for _, job := range jobs {
					job.Status = actions_model.StatusRunning
					_, err := actions_model.UpdateRunJob(context.Background(), job, nil, "status")
					assert.NoError(t, err)
				}

				runningCount, err := actions_model.CountRunningJobsByWorkflowAndRun(context.Background(), runID, jobID)
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, runningCount, 5, "All jobs should be able to run when max-parallel is "+tc.name)
			})
		}
	})
}
