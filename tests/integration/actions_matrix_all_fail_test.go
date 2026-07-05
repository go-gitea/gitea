// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"sync"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestActionsMatrixAllLegsFailAggregates is the regression test for issue #38333: when every
// leg of a fail-fast:false matrix finishes at the same time, all reporting failure, the run
// must aggregate to failure instead of staying stuck in running.
//
// It lives in tests/integration rather than a unit test because the underlying bug is a
// write-skew between the concurrent run-status aggregations of each finishing leg, which only
// occurs under a real database's snapshot isolation (MySQL's default REPEATABLE READ). SQLite
// serializes write transactions, so this test can only fail against the MySQL/PostgreSQL CI lanes.
func TestActionsMatrixAllLegsFailAggregates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	const legs = 5

	run := &actions_model.ActionRun{
		Title:         "matrix-all-fail",
		RepoID:        1,
		OwnerID:       2,
		WorkflowID:    "matrix.yaml",
		Index:         9910,
		TriggerUserID: 2,
		Ref:           "refs/heads/main",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       1,
		TriggerUserID: 2,
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, attempt))
	run.LatestAttemptID = attempt.ID
	require.NoError(t, actions_model.UpdateRun(ctx, run, "latest_attempt_id"))

	// One waiting job per matrix leg, all sharing the JobID like a real matrix expansion.
	jobs := make([]*actions_model.ActionRunJob, legs)
	for i := range legs {
		jobs[i] = &actions_model.ActionRunJob{
			RunID:           run.ID,
			RunAttemptID:    attempt.ID,
			RepoID:          run.RepoID,
			OwnerID:         run.OwnerID,
			CommitSHA:       run.CommitSHA,
			Name:            "checks",
			Attempt:         1,
			JobID:           "checks",
			Status:          actions_model.StatusWaiting,
			RunsOn:          []string{"ubuntu-latest"},
			WorkflowPayload: minimalConcurrentWorkflowPayload("checks"),
		}
		require.NoError(t, db.Insert(ctx, jobs[i]))
	}

	// One runner per leg claims a distinct job, mirroring a fail-fast:false matrix where every
	// leg runs to completion on its own runner.
	tasks := make([]*actions_model.ActionTask, legs)
	for i := range legs {
		r := &actions_model.ActionRunner{
			UUID:        "matrix-runner-uuid-" + string(rune('a'+i)),
			Name:        "matrix-runner-" + string(rune('a'+i)),
			AgentLabels: []string{"ubuntu-latest"},
		}
		r.GenerateAndFillToken()
		require.NoError(t, db.Insert(ctx, r))

		task, ok, err := actions_model.CreateTaskForRunner(ctx, r)
		require.NoError(t, err)
		require.True(t, ok, "runner %d did not claim a job", i)
		tasks[i] = task
	}

	// Every leg reports failure at once through the real runner-reporting entry point. The
	// start barrier releases them together to maximize transaction overlap, which is what
	// triggers the aggregation write-skew under snapshot isolation.
	stoppedAt := timestamppb.New(time.Now())
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, legs)
	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task *actions_model.ActionTask) {
			defer wg.Done()
			<-start
			_, errs[i] = actions_model.UpdateTaskByState(ctx, task.RunnerID, &runnerv1.TaskState{
				Id:        task.ID,
				Result:    runnerv1.Result_RESULT_FAILURE,
				StoppedAt: stoppedAt,
			})
		}(i, task)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		require.NoErrorf(t, err, "leg %d failed to report", i)
	}

	gotRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	assert.Equal(t, actions_model.StatusFailure, gotRun.Status, "run must aggregate to failure once all legs fail")

	gotAttempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: attempt.ID})
	assert.Equal(t, actions_model.StatusFailure, gotAttempt.Status)
}
