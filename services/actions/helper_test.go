// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmitJob(t *testing.T) {
	checked, evaluated := 0, 0
	badRunsOn := func() error { checked++; return errors.New("bad runs-on") }
	evaluate := func(job *actions_model.ActionRunJob) func() error {
		return func() error { evaluated++; job.IsConcurrencyEvaluated = true; return nil }
	}
	slots := maxParallelSlots{}
	newJob := func() *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{JobID: "build", Status: actions_model.StatusWaiting, MaxParallel: 1, RawConcurrency: "group: g"}
	}

	first := newJob()
	invalid, cancelled, err := admitJob(t.Context(), first, slots, nil, evaluate(first))
	require.NoError(t, err)
	require.NoError(t, invalid)
	assert.Empty(t, cancelled)
	assert.Equal(t, actions_model.StatusWaiting, first.Status)
	assert.Equal(t, 1, evaluated)

	// the slot is taken: a starved job neither checks its runs-on nor evaluates its concurrency
	starved := newJob()
	invalid, _, err = admitJob(t.Context(), starved, slots, badRunsOn, evaluate(starved))
	require.NoError(t, err)
	require.NoError(t, invalid)
	assert.Equal(t, actions_model.StatusBlocked, starved.Status)
	assert.Equal(t, 0, checked)
	assert.Equal(t, 1, evaluated)

	failed := newJob()
	invalid, _, err = admitJob(t.Context(), failed, maxParallelSlots{}, badRunsOn, evaluate(failed))
	require.NoError(t, err)
	assert.ErrorContains(t, invalid, "bad runs-on")
	assert.Equal(t, actions_model.StatusFailure, failed.Status)
	assert.NotZero(t, failed.Stopped)
	assert.Equal(t, 1, evaluated)

	// a job without `concurrency:` has nothing to evaluate
	plain := newJob()
	plain.RawConcurrency = ""
	_, _, err = admitJob(t.Context(), plain, maxParallelSlots{}, nil, evaluate(plain))
	require.NoError(t, err)
	assert.Equal(t, 1, evaluated)

	_, _, err = admitJob(t.Context(), newJob(), maxParallelSlots{}, nil, func() error { return errors.New("bad concurrency") })
	assert.ErrorContains(t, err, "bad concurrency")
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
	mid := insertJob("mid", caller.ID, "", `{"inputs":{"env":"leaf"}}`)
	leaf := insertJob("leaf", mid.ID, "", "")

	dispatchRun := &actions_model.ActionRun{ID: runID, Event: "workflow_dispatch", EventPayload: `{"inputs":{"flag":"true","Shared":"from-dispatch"}}`}
	pushRun := &actions_model.ActionRun{ID: runID, Event: "push", EventPayload: `{}`}

	t.Run("dispatch inputs overlaid case-insensitively with the caller's with", func(t *testing.T) {
		inputs, err := getInputsForJob(ctx, dispatchRun, mid)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"flag": true, "shared": "from-call", "mid_only": "mid"}, inputs)
	})

	t.Run("task context of a nested child keeps the older runners' form and carries the original event", func(t *testing.T) {
		leaf.Run = dispatchRun
		gitCtx := GiteaContext{
			"event_name": "workflow_dispatch",
			"event":      map[string]any{"inputs": map[string]any{"flag": "true", "Shared": "from-dispatch"}},
		}
		require.NoError(t, setCalledWorkflowContext(ctx, leaf, gitCtx))
		inputs := map[string]any{"flag": true, "Shared": "from-dispatch", "env": "leaf"}
		assert.Equal(t, "workflow_call", gitCtx["event_name"])
		assert.Equal(t, map[string]any{"inputs": inputs}, gitCtx["event"])
		assert.Equal(t, map[string]any{
			"original_event_name":   "workflow_dispatch",
			"original_event_inputs": map[string]any{"flag": "true", "Shared": "from-dispatch"},
			"inputs":                inputs,
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

func TestPullRequestTargetBaseSHA(t *testing.T) {
	prPayload := func(baseSHA string) string {
		payload, err := json.Marshal(api.PullRequestPayload{
			PullRequest: &api.PullRequest{
				Base: &api.PRBranchInfo{Sha: baseSHA},
			},
		})
		require.NoError(t, err)
		return string(payload)
	}

	for _, test := range []struct {
		name         string
		triggerEvent string
		payload      string
		wantSHA      string
	}{
		{"pull_request_target with base SHA", actions_module.GithubEventPullRequestTarget, prPayload("base-sha"), "base-sha"},
		{"non pull_request_target trigger", actions_module.GithubEventPullRequest, prPayload("base-sha"), ""},
		{"missing base SHA", actions_module.GithubEventPullRequestTarget, prPayload(""), ""},
		{"invalid payload", actions_module.GithubEventPullRequestTarget, "{", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := pullRequestTargetBaseSHA(&actions_model.ActionRun{
				Event:        webhook_module.HookEventPullRequest,
				TriggerEvent: test.triggerEvent,
				EventPayload: test.payload,
			})
			if test.wantSHA != "" {
				assert.True(t, ok)
				assert.Equal(t, test.wantSHA, got)
			} else {
				assert.False(t, ok)
				assert.Empty(t, got)
			}
		})
	}
}
