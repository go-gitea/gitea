// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateJobIfDefersMatrixExpression(t *testing.T) {
	// A placeholder's `if:` reading `matrix.*` can only be decided per combination once the matrix is expanded.

	emptyRun := &actions_model.ActionRun{} // if the gate is removed, the emptyRun will cause an error
	deferredJob := func(ifExpr string) *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{
			ID: 1, JobID: "build", Needs: []string{"setup"}, IsMatrixDeferred: true,
			WorkflowPayload: fmt.Appendf(nil, `name: test
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    if: %s
    strategy:
      matrix:
        value: ${{ fromJson(needs.setup.outputs.m) }}
    steps:
      - run: echo
`, ifExpr),
		}
	}

	for _, tt := range []struct {
		ifExpr string
		// wantNeedsFailed is what the `if:` decides when the needs did not all succeed.
		wantNeedsFailed bool
	}{
		// These hold only for a real combination, so none can be decided before the matrix expands,
		// and the fallback is the needs gate: a job whose needs did not succeed is still skipped.
		{ifExpr: "${{ matrix.value == 1 }}"},
		{ifExpr: "matrix.value == 1"}, // an `if:` may omit the `${{ }}`
		// always() asks to run whatever the needs did, so it must not fall back to their gate.
		{ifExpr: "${{ always() && matrix.value == 1 }}", wantNeedsFailed: true},
	} {
		t.Run(tt.ifExpr, func(t *testing.T) {
			got, err := evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(tt.ifExpr), nil, true)
			require.NoError(t, err)
			assert.True(t, got, "must reach the expansion that gates each combination on its own values")

			got, err = evaluateJobIf(t.Context(), emptyRun, nil, deferredJob(tt.ifExpr), nil, false)
			require.NoError(t, err)
			assert.Equal(t, tt.wantNeedsFailed, got)
		})
	}
}

func TestGetWorkflowDispatchInputsFromRunCoercesBooleans(t *testing.T) {
	// Regression: a workflow_dispatch boolean input is stored as the string "true"/"false" in
	// run.EventPayload (github.event.inputs must stay a string, matching GitHub). The `inputs`
	// context used by a needs-gated job's server-side `if:` evaluation must still see it as a
	// real boolean, or `if: ${{ inputs.deploy == true }}` never matches and the job stays
	// blocked forever - the job's own WorkflowPayload carries the `on:` declaration needed to
	// tell which inputs are booleans.
	run := &actions_model.ActionRun{
		Event: "workflow_dispatch",
		EventPayload: func() string {
			payload, err := json.Marshal(api.WorkflowDispatchPayload{
				Inputs: map[string]any{"deploy": "true", "env": "prod"},
			})
			require.NoError(t, err)
			return string(payload)
		}(),
	}
	job := &actions_model.ActionRunJob{
		ID: 1, JobID: "deploy",
		WorkflowPayload: []byte(`name: test
on:
  workflow_dispatch:
    inputs:
      deploy:
        type: boolean
        default: true
      env:
        type: string
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`),
	}

	inputs, err := getWorkflowDispatchInputsFromRun(run, job)
	require.NoError(t, err)
	assert.Equal(t, true, inputs["deploy"])
	assert.Equal(t, "prod", inputs["env"])

	// github.event.inputs (the raw event payload) must be left untouched: still the dispatch string.
	var payload api.WorkflowDispatchPayload
	require.NoError(t, json.Unmarshal([]byte(run.EventPayload), &payload))
	assert.Equal(t, "true", payload.Inputs["deploy"])

	// job == nil (workflow-level concurrency, no job in scope) skips coercion rather than failing.
	inputs, err = getWorkflowDispatchInputsFromRun(run, nil)
	require.NoError(t, err)
	assert.Equal(t, "true", inputs["deploy"])
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
