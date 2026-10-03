// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// minimalWorkflowPayload returns the minimal YAML for a single-job workflow with no steps.
func minimalConcurrentWorkflowPayload(jobID string) []byte {
	return []byte("on: push\njobs:\n  " + jobID + ":\n    runs-on: ubuntu-latest\n")
}

// TestCreateTaskForRunnerConcurrentClaim verifies that when multiple runners
// poll simultaneously and all initially see the same first waiting job,
// each runner claims a distinct job rather than all but one being left
// empty-handed. This is the regression test for the race condition where
// runners losing the optimistic-lock on job #1 would receive latestVersion
// and never retry the remaining 49+ jobs.
//
// It lives in tests/integration rather than a unit test because SQLite
// serializes write transactions, so the contended optimistic-lock path this
// guards only runs concurrently against MySQL/PostgreSQL in CI.
func TestCreateTaskForRunnerConcurrentClaim(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const numJobs = 3

	run := &actions_model.ActionRun{
		Title:         "concurrent-claim-test-run",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "test.yaml",
		Index:         9901,
		TriggerUserID: 2,
		Ref:           "refs/heads/main",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		Status:        actions_model.StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))

	jobs := make([]*actions_model.ActionRunJob, numJobs)
	for i := range numJobs {
		jobID := "concurrent-job-" + string(rune('a'+i))
		jobs[i] = &actions_model.ActionRunJob{
			RunID:           run.ID,
			RepoID:          run.RepoID,
			OwnerID:         run.OwnerID,
			CommitSHA:       run.CommitSHA,
			Name:            jobID,
			Attempt:         1,
			JobID:           jobID,
			Status:          actions_model.StatusWaiting,
			RunsOn:          []string{"ubuntu-latest"},
			WorkflowPayload: minimalConcurrentWorkflowPayload(jobID),
		}
		require.NoError(t, db.Insert(t.Context(), jobs[i]))
	}

	runners := make([]*actions_model.ActionRunner, numJobs)
	for i := range numJobs {
		r := &actions_model.ActionRunner{
			UUID:        "concurrent-runner-uuid-" + string(rune('a'+i)),
			Name:        "concurrent-runner-" + string(rune('a'+i)),
			AgentLabels: []string{"ubuntu-latest"},
		}
		r.GenerateAndFillToken()
		runners[i] = r
		require.NoError(t, db.Insert(t.Context(), runners[i]))
	}

	// Simulate the burst: all runners call CreateTaskForRunner concurrently,
	// as happens when all see the same stale tasksVersion simultaneously.
	type result struct {
		task *actions_model.ActionTask
		ok   bool
		err  error
	}
	results := make([]result, numJobs)
	var wg sync.WaitGroup
	for i := range numJobs {
		wg.Go(func() {
			task, ok, err := actions_model.CreateTaskForRunner(t.Context(), runners[i])
			results[i] = result{task, ok, err}
		})
	}
	wg.Wait()

	// Every runner must have received a task without error.
	claimedJobIDs := make(map[int64]bool)
	for i, r := range results {
		require.NoError(t, r.err, "runner %d got an unexpected error", i)
		require.True(t, r.ok, "runner %d did not get a task even though free jobs exist", i)
		require.NotNil(t, r.task)
		assert.False(t, claimedJobIDs[r.task.JobID], "job %d was claimed by more than one runner", r.task.JobID)
		claimedJobIDs[r.task.JobID] = true
	}
	assert.Len(t, claimedJobIDs, numJobs, "expected %d distinct jobs to be claimed", numJobs)

	// All jobs must now be running with a task assigned.
	for _, j := range jobs {
		updated := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: j.ID})
		assert.Equal(t, actions_model.StatusRunning, updated.Status)
		assert.NotZero(t, updated.TaskID)
	}
}

func prepareWaitingRunJob(t *testing.T) *actions_model.ActionRunJob {
	if setting.Database.Type.IsSQLite3() {
		t.Skip("SQLite serializes write transactions")
	}
	job := &actions_model.ActionRunJob{RepoID: 1, Status: actions_model.StatusWaiting, RunsOn: []string{"ubuntu-latest"}}
	require.NoError(t, db.Insert(t.Context(), job))
	return job
}

func TestCreateTaskForRunnerDuringOpenClaimDoesNotWait(t *testing.T) {
	job := prepareWaitingRunJob(t)

	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		_, err := db.GetEngine(ctx).ID(job.ID).Cols("name").Update(&actions_model.ActionRunJob{Name: "claiming"})
		require.NoError(t, err)
		pickupCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		_, _, err = actions_model.CreateTaskForRunner(pickupCtx, &actions_model.ActionRunner{})
		return err
	}))
}

func TestClaimRunJobAfterConcurrentCancelUpdatesNothing(t *testing.T) {
	job := prepareWaitingRunJob(t)

	require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
		claimed, err := actions_model.GetRunJobByRepoAndID(ctx, job.RepoID, job.ID)
		require.NoError(t, err)
		_, err = db.GetEngine(t.Context()).ID(job.ID).Cols("status").Update(&actions_model.ActionRunJob{Status: actions_model.StatusCancelled})
		require.NoError(t, err)

		claimed.TaskID, claimed.Status = 1, actions_model.StatusRunning
		affected, err := actions_model.UpdateRunJob(ctx, claimed, builder.Eq{"task_id": 0, "status": actions_model.StatusWaiting}, "task_id", "status")
		require.NoError(t, err)
		assert.Zero(t, affected)
		return nil
	}))
}
