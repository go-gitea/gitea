// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmitJob(t *testing.T) {
	checked, evaluated := 0, 0
	badRunsOn, badConcurrency := errors.New("bad runs-on"), errors.New("bad concurrency")
	checkRunsOn := func() error { checked++; return badRunsOn }
	evaluate := func(job *actions_model.ActionRunJob) func() error {
		return func() error { evaluated++; job.IsConcurrencyEvaluated = true; return nil }
	}
	newJob := func() *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{JobID: "build", Status: actions_model.StatusWaiting, MaxParallel: 1, RawConcurrency: "group: g"}
	}

	t.Run("a full slot blocks the next job before runs-on and concurrency checks", func(t *testing.T) {
		slots := maxParallelSlots{}
		first := newJob()
		cancelled, runsOnErr, err := admitJob(t.Context(), first, slots, nil, evaluate(first))
		require.NoError(t, err)
		require.NoError(t, runsOnErr)
		assert.Empty(t, cancelled)
		assert.Equal(t, actions_model.StatusWaiting, first.Status)

		starved := newJob()
		_, runsOnErr, err = admitJob(t.Context(), starved, slots, checkRunsOn, evaluate(starved))
		require.NoError(t, err)
		require.NoError(t, runsOnErr)
		assert.Equal(t, actions_model.StatusBlocked, starved.Status)
		assert.Equal(t, 0, checked)
		assert.Equal(t, 1, evaluated)
	})

	t.Run("an invalid runs-on fails the job before concurrency", func(t *testing.T) {
		failed := newJob()
		_, runsOnErr, err := admitJob(t.Context(), failed, maxParallelSlots{}, checkRunsOn, evaluate(failed))
		require.NoError(t, err)
		assert.ErrorIs(t, runsOnErr, badRunsOn)
		assert.Equal(t, actions_model.StatusFailure, failed.Status)
		assert.NotZero(t, failed.Stopped)
		assert.False(t, failed.IsConcurrencyEvaluated)
	})

	t.Run("a job without concurrency skips evaluation", func(t *testing.T) {
		plain := newJob()
		plain.RawConcurrency = ""
		_, _, err := admitJob(t.Context(), plain, maxParallelSlots{}, nil, evaluate(plain))
		require.NoError(t, err)
		assert.False(t, plain.IsConcurrencyEvaluated)
	})

	t.Run("a concurrency evaluation error is returned", func(t *testing.T) {
		_, _, err := admitJob(t.Context(), newJob(), maxParallelSlots{}, nil, func() error { return badConcurrency })
		assert.ErrorIs(t, err, badConcurrency)
	})
}

func TestDispatchInputsForRunJobs(t *testing.T) {
	// a child carries the callee's `on: workflow_call`, so only a top-level job answers for the run
	run := &actions_model.ActionRun{Event: "workflow_dispatch", EventPayload: `{"inputs":{"deploy":"true"}}`}
	job := &actions_model.ActionRunJob{
		ID: 1, JobID: "deploy",
		WorkflowPayload: []byte("on: {workflow_dispatch: {inputs: {deploy: {type: boolean}}}}\njobs:\n  deploy:\n    steps: [{run: echo}]\n"),
	}
	child := &actions_model.ActionRunJob{
		ID: 2, JobID: "called", ParentJobID: job.ID,
		WorkflowPayload: []byte("on: workflow_call\njobs:\n  called:\n    steps: [{run: echo}]\n"),
	}

	inputs, err := dispatchInputsForRunJobs(run, []*actions_model.ActionRunJob{child, job})
	require.NoError(t, err)
	assert.Equal(t, true, inputs["deploy"])
}

func TestReusableChildInputs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const runID = 9801
	insertJob := func(jobID string, parentID int64, payload, callPayload string) *actions_model.ActionRunJob {
		job := &actions_model.ActionRunJob{RunID: runID, JobID: jobID, ParentJobID: parentID, WorkflowPayload: []byte(payload), CallPayload: callPayload}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}
	caller := insertJob("caller", 0,
		"on: {workflow_dispatch: {inputs: {flag: {type: boolean}, Shared: {type: string}}}}\njobs:\n  caller:\n    uses: ./.gitea/workflows/mid.yml\n",
		`{"inputs":{"shared":"from-call","mid_only":"mid"}}`)
	mid := insertJob("mid", caller.ID, "", `{"inputs":{"shared":"from-mid","env":"leaf"}}`)
	leaf := insertJob("leaf", mid.ID, "", "")

	dispatchRun := &actions_model.ActionRun{ID: runID, Event: "workflow_dispatch", EventPayload: `{"inputs":{"flag":"true","Shared":"from-dispatch"}}`}
	pushRun := &actions_model.ActionRun{ID: runID, Event: "push", EventPayload: `{}`}
	dispatchInputs := map[string]any{"flag": true, "shared": "from-mid", "env": "leaf"}

	t.Run("preloaded ancestors supply top-level dispatch inputs and overlay direct caller inputs case-insensitively", func(t *testing.T) {
		inputs, err := getInputsForJobWithAncestors(ctx, dispatchRun, leaf, []*actions_model.ActionRunJob{mid, caller})
		require.NoError(t, err)
		assert.Equal(t, dispatchInputs, inputs)
	})

	t.Run("task context of a nested child keeps the older runners' form and carries the original event", func(t *testing.T) {
		leaf.Run = dispatchRun
		gitCtx := GiteaContext{
			"event_name": "workflow_dispatch",
			"event":      map[string]any{"inputs": map[string]any{"flag": "true", "Shared": "from-dispatch"}},
		}
		require.NoError(t, setCalledWorkflowContext(ctx, leaf, gitCtx))
		assert.Equal(t, "workflow_call", gitCtx["event_name"])
		assert.Equal(t, map[string]any{"inputs": dispatchInputs}, gitCtx["event"])
		assert.Equal(t, map[string]any{
			"original_event_name":   "workflow_dispatch",
			"original_event_inputs": map[string]any{"flag": "true", "Shared": "from-dispatch"},
			"inputs":                dispatchInputs,
		}, gitCtx["gitea_workflow_call"])
	})

	t.Run("task context of a non-dispatch run", func(t *testing.T) {
		mid.Run = pushRun
		gitCtx := GiteaContext{"event_name": "push", "event": map[string]any{}}
		require.NoError(t, setCalledWorkflowContext(ctx, mid, gitCtx))
		inputs := map[string]any{"shared": "from-call", "mid_only": "mid"}
		assert.Equal(t, "workflow_call", gitCtx["event_name"])
		assert.Equal(t, map[string]any{"inputs": inputs}, gitCtx["event"])
		assert.Equal(t, map[string]any{"original_event_name": "push", "inputs": inputs}, gitCtx["gitea_workflow_call"])
	})
}
