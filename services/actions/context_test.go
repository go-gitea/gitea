// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	act_model "gitea.com/gitea/runner/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRunConcurrency_RunIDFallback(t *testing.T) {
	// Unit-level check that EvaluateRunConcurrencyFillModel resolves github.run_id from run.ID.
	// The full-flow regression (run.ID non-zero by evaluation time) is TestPrepareRunAndInsert_ExpressionsSeeRunID.
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
	// Regression for the cross-branch concurrency leak: github.run_id must be available during both
	// jobparser.Parse (run-name) and concurrency evaluation; inserting run after either leaves run.ID at 0.
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

func TestComputeReusableCallerOutputs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	var nextRunIndex int64 = 9001
	insertRun := func(t *testing.T, workflowID string) *actions_model.ActionRun {
		t.Helper()
		run := &actions_model.ActionRun{
			Title:         "reusable-out",
			RepoID:        4,
			Index:         nextRunIndex,
			OwnerID:       1,
			WorkflowID:    workflowID,
			TriggerUserID: 1,
			Ref:           "refs/heads/master",
			CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event:         "push",
			TriggerEvent:  "push",
			EventPayload:  "{}",
			Status:        actions_model.StatusSuccess,
		}
		nextRunIndex++
		require.NoError(t, db.Insert(ctx, run))
		return run
	}

	insertCaller := func(t *testing.T, run *actions_model.ActionRun, jobID string, parentID int64, content, callPayload string) *actions_model.ActionRunJob {
		t.Helper()
		job := &actions_model.ActionRunJob{
			RunID:                   run.ID,
			RepoID:                  run.RepoID,
			OwnerID:                 run.OwnerID,
			CommitSHA:               run.CommitSHA,
			Name:                    jobID,
			JobID:                   jobID,
			Attempt:                 1,
			Status:                  actions_model.StatusSuccess,
			ParentJobID:             parentID,
			IsReusableCaller:        true,
			IsExpanded:              true,
			ReusableWorkflowContent: []byte(content),
			CallPayload:             callPayload,
		}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}

	// Each call to insertChildJobAndTask with non-empty outputs allocates a fresh TaskID
	// so its action_task_output rows stay isolated per subtest.
	var nextTaskID int64 = 90001
	insertChildJobAndTask := func(t *testing.T, run *actions_model.ActionRun, jobID string, parentID int64, outputs map[string]string) *actions_model.ActionRunJob {
		t.Helper()
		var taskID int64
		if len(outputs) > 0 {
			taskID = nextTaskID
			nextTaskID++
		}
		job := &actions_model.ActionRunJob{
			RunID:       run.ID,
			RepoID:      run.RepoID,
			OwnerID:     run.OwnerID,
			CommitSHA:   run.CommitSHA,
			Name:        jobID,
			JobID:       jobID,
			Attempt:     1,
			Status:      actions_model.StatusSuccess,
			ParentJobID: parentID,
			TaskID:      taskID,
		}
		require.NoError(t, db.Insert(ctx, job))
		for k, v := range outputs {
			require.NoError(t, db.Insert(ctx, &actions_model.ActionTaskOutput{
				TaskID:      taskID,
				OutputKey:   k,
				OutputValue: v,
			}))
		}
		return job
	}

	// childrenByParentOfRun returns the run's jobs indexed by ParentJobID, the shape computeReusableCallerOutputs expects.
	childrenByParentOfRun := func(t *testing.T, runID int64) map[int64][]*actions_model.ActionRunJob {
		t.Helper()
		all, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: runID})
		require.NoError(t, err)
		index := make(map[int64][]*actions_model.ActionRunJob)
		for _, j := range all {
			if j.ParentJobID != 0 {
				index[j.ParentJobID] = append(index[j.ParentJobID], j)
			}
		}
		return index
	}

	t.Run("returns empty when callee declares no outputs", func(t *testing.T) {
		run := insertRun(t, "no-outputs.yaml")
		caller := insertCaller(t, run, "caller", 0, `on:
  workflow_call:
    outputs: {}
`, "")
		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("unexpanded (skipped) caller yields empty outputs without error", func(t *testing.T) {
		run := insertRun(t, "skipped-caller.yaml")
		// A reusable caller skipped before expansion: IsExpanded=false, empty ReusableWorkflowContent, no children.
		caller := &actions_model.ActionRunJob{
			RunID:            run.ID,
			RepoID:           run.RepoID,
			OwnerID:          run.OwnerID,
			CommitSHA:        run.CommitSHA,
			Name:             "caller",
			JobID:            "caller",
			Attempt:          1,
			Status:           actions_model.StatusSkipped,
			IsReusableCaller: true,
			IsExpanded:       false,
		}
		require.NoError(t, db.Insert(ctx, caller))
		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("literal output value passes through", func(t *testing.T) {
		run := insertRun(t, "literal-out.yaml")
		caller := insertCaller(t, run, "caller", 0, `on:
  workflow_call:
    outputs:
      hello:
        value: world
`, "")
		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"hello": "world"}, out)
	})

	t.Run("output expression reads child task outputs", func(t *testing.T) {
		run := insertRun(t, "child-out.yaml")
		caller := insertCaller(t, run, "caller", 0, `on:
  workflow_call:
    outputs:
      result:
        value: ${{ jobs.child.outputs.foo }}
`, "")
		insertChildJobAndTask(t, run, "child", caller.ID, map[string]string{"foo": "bar"})

		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"result": "bar"}, out)
	})

	t.Run("CallPayload inputs reachable in output expression", func(t *testing.T) {
		run := insertRun(t, "payload-out.yaml")
		payload, err := json.Marshal(api.WorkflowCallPayload{
			Inputs: map[string]any{"env": "staging"},
		})
		require.NoError(t, err)
		caller := insertCaller(t, run, "caller", 0, `on:
  workflow_call:
    inputs:
      env:
        type: string
    outputs:
      env:
        value: ${{ inputs.env }}
`, string(payload))

		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"env": "staging"}, out)
	})

	t.Run("nested caller outputs propagate to outer", func(t *testing.T) {
		run := insertRun(t, "nested-out.yaml")
		outer := insertCaller(t, run, "outer", 0, `on:
  workflow_call:
    outputs:
      bubbled:
        value: ${{ jobs.inner.outputs.up }}
`, "")
		inner := insertCaller(t, run, "inner", outer.ID, `on:
  workflow_call:
    outputs:
      up:
        value: ${{ jobs.leaf.outputs.foo }}
`, "")
		insertChildJobAndTask(t, run, "leaf", inner.ID, map[string]string{"foo": "bubble-value"})

		out, err := computeReusableCallerOutputs(ctx, outer, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"bubbled": "bubble-value"}, out)
	})

	t.Run("matrix children with same JobID prefer non-empty values", func(t *testing.T) {
		run := insertRun(t, "matrix-out.yaml")
		caller := insertCaller(t, run, "caller", 0, `on:
  workflow_call:
    outputs:
      foo:
        value: ${{ jobs.matrix.outputs.foo }}
`, "")
		insertChildJobAndTask(t, run, "matrix", caller.ID, map[string]string{"foo": ""})
		insertChildJobAndTask(t, run, "matrix", caller.ID, map[string]string{"foo": "filled"})

		out, err := computeReusableCallerOutputs(ctx, caller, childrenByParentOfRun(t, run.ID))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"foo": "filled"}, out)
	})
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
