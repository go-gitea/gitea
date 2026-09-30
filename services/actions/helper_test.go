// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
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

	t.Run("pull_request_target with base SHA", func(t *testing.T) {
		run := &actions_model.ActionRun{
			Event:        webhook_module.HookEventPullRequest,
			TriggerEvent: actions_module.GithubEventPullRequestTarget,
			EventPayload: prPayload("base-sha"),
		}
		got, ok := pullRequestTargetBaseSHA(run)
		assert.True(t, ok)
		assert.Equal(t, "base-sha", got)
	})

	t.Run("non pull_request_target trigger", func(t *testing.T) {
		run := &actions_model.ActionRun{
			Event:        webhook_module.HookEventPullRequest,
			TriggerEvent: actions_module.GithubEventPullRequest,
			EventPayload: prPayload("base-sha"),
		}
		got, ok := pullRequestTargetBaseSHA(run)
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("missing base SHA", func(t *testing.T) {
		run := &actions_model.ActionRun{
			Event:        webhook_module.HookEventPullRequest,
			TriggerEvent: actions_module.GithubEventPullRequestTarget,
			EventPayload: prPayload(""),
		}
		got, ok := pullRequestTargetBaseSHA(run)
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("invalid payload", func(t *testing.T) {
		run := &actions_model.ActionRun{
			Event:        webhook_module.HookEventPullRequest,
			TriggerEvent: actions_module.GithubEventPullRequestTarget,
			EventPayload: "{",
		}
		got, ok := pullRequestTargetBaseSHA(run)
		assert.False(t, ok)
		assert.Empty(t, got)
	})
}
