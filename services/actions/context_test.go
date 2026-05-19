// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"

	act_model "github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRunConcurrency_RunIDFallback(t *testing.T) {
	// Unit-level check that EvaluateRunConcurrencyFillModel resolves
	// github.run_id from run.ID. The full-flow regression — that run.ID is
	// non-zero by the time evaluation happens — is in
	// TestPrepareRunAndInsert_ExpressionsSeeRunID.
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	runA := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 791})
	runB := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 792})

	attemptA := &actions_model.ActionRunAttempt{RepoID: runA.RepoID, RunID: runA.ID, Attempt: 1}
	attemptB := &actions_model.ActionRunAttempt{RepoID: runB.RepoID, RunID: runB.ID, Attempt: 1}

	expr := &act_model.RawConcurrency{
		Group:            "${{ github.workflow }}-${{ github.head_ref || github.run_id }}",
		CancelInProgress: "true",
	}

	assert.NoError(t, EvaluateRunConcurrencyFillModel(ctx, runA, attemptA, expr, nil, nil))
	assert.NoError(t, EvaluateRunConcurrencyFillModel(ctx, runB, attemptB, expr, nil, nil))

	assert.Contains(t, attemptA.ConcurrencyGroup, "791")
	assert.Contains(t, attemptB.ConcurrencyGroup, "792")
	assert.NotEqual(t, attemptA.ConcurrencyGroup, attemptB.ConcurrencyGroup)
}

func TestPrepareRunAndInsert_ExpressionsSeeRunID(t *testing.T) {
	// Regression for the cross-branch concurrency leak: github.run_id must
	// be available during BOTH jobparser.Parse (run-name) and workflow-level
	// concurrency evaluation. Re-ordering db.Insert relative to either step
	// would leave run.ID at 0 and break this test.
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	content := []byte(`name: cross-branch
run-name: "Run ${{ github.run_id }}"
on: push
concurrency:
  group: group-${{ github.run_id }}
  cancel-in-progress: true
jobs:
  hello:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`)

	run := &actions_model.ActionRun{
		Title:         "before parse",
		RepoID:        4,
		OwnerID:       1,
		WorkflowID:    "expr-runid.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
	}
	require.NoError(t, PrepareRunAndInsert(ctx, content, run, nil))
	require.Positive(t, run.ID)

	persisted := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	runIDStr := strconv.FormatInt(run.ID, 10)
	assert.Equal(t, "Run "+runIDStr, persisted.Title)
	// ConcurrencyGroup lives on the latest attempt after migration v331.
	require.Positive(t, persisted.LatestAttemptID)
	attempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: persisted.LatestAttemptID})
	assert.Equal(t, "group-"+runIDStr, attempt.ConcurrencyGroup)
	// Rerun reads raw_concurrency from the DB to re-evaluate the group;
	// see services/actions/rerun.go. Must survive the insert.
	assert.NotEmpty(t, persisted.RawConcurrency)
}

func TestFindTaskNeeds(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 51})
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})

	ret, err := FindTaskNeeds(t.Context(), job)
	assert.NoError(t, err)
	assert.Len(t, ret, 1)
	assert.Contains(t, ret, "job1")
	assert.Len(t, ret["job1"].Outputs, 2)
	assert.Equal(t, "abc", ret["job1"].Outputs["output_a"])
	assert.Equal(t, "bbb", ret["job1"].Outputs["output_b"])
}
