// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
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
