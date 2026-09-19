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
