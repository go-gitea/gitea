// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveJobEnvironment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	const repoID = 4

	_, _, err := GetOrCreateEnvironment(ctx, repoID, "production", []string{"main"})
	require.NoError(t, err)

	resolve := func(environment string, run *actions_model.ActionRun) (*actions_model.ActionEnvironment, string) {
		env, denyReason, err := ResolveJobEnvironment(ctx, &actions_model.ActionRunJob{RepoID: repoID, EnvironmentName: environment, Run: run})
		require.NoError(t, err)
		return env, denyReason
	}
	push := func(ref string) *actions_model.ActionRun {
		return &actions_model.ActionRun{RepoID: repoID, Ref: ref, TriggerEvent: "push"}
	}

	t.Run("allowed ref", func(t *testing.T) {
		env, denyReason := resolve("PRODUCTION", push("refs/heads/main"))
		require.NotNil(t, env)
		assert.Empty(t, denyReason)
	})
	t.Run("ref refused by the policy", func(t *testing.T) {
		_, denyReason := resolve("production", push("refs/heads/feature"))
		assert.Contains(t, denyReason, "not allowed")
	})
	t.Run("pull_request_target is judged by its base branch", func(t *testing.T) {
		run := &actions_model.ActionRun{
			RepoID: repoID, Ref: "refs/pull/1/head", Event: "pull_request", TriggerEvent: "pull_request_target",
			EventPayload: `{"pull_request":{"base":{"ref":"main"},"head":{"ref":"feature"}}}`,
		}
		_, denyReason := resolve("production", run)
		assert.Empty(t, denyReason)
	})
	t.Run("invalid name", func(t *testing.T) {
		env, denyReason := resolve(strings.Repeat("x", actions_model.EnvironmentNameMaxLength+1), push("refs/heads/main"))
		assert.Nil(t, env)
		assert.Contains(t, denyReason, "invalid")
	})
	t.Run("untrusted fork run", func(t *testing.T) {
		run := push("refs/pull/1/head")
		run.IsForkPullRequest, run.TriggerEvent = true, "pull_request"
		env, denyReason := resolve("production", run)
		assert.Nil(t, env)
		assert.Empty(t, denyReason)

		EnsureEnvironments(ctx, run, []*actions_model.ActionRunJob{{RepoID: repoID, EnvironmentName: "from-fork"}})
		unittest.AssertNotExistsBean(t, &actions_model.ActionEnvironment{RepoID: repoID, LowerName: "from-fork"})
	})
}

func TestDeniedJobKeepsEphemeralRunner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(int64) error { return nil })()

	_, _, err := GetOrCreateEnvironment(ctx, 1, "production", []string{"main"})
	require.NoError(t, err)
	run := &actions_model.ActionRun{
		Title: "denied-run", RepoID: 1, OwnerID: 2, WorkflowID: "test.yaml", TriggerUserID: 2,
		Ref: "refs/heads/feature", CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event: "push", TriggerEvent: "push", Status: actions_model.StatusWaiting,
	}
	require.NoError(t, db.Insert(ctx, run))
	job := &actions_model.ActionRunJob{
		RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
		Name: "deploy", Attempt: 1, JobID: "deploy", Status: actions_model.StatusWaiting,
		RunsOn: []string{"ubuntu-latest"}, EnvironmentName: "production",
		WorkflowPayload: []byte("on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: production\n    steps:\n      - run: echo hi\n"),
	}
	require.NoError(t, db.Insert(ctx, job))
	runner := &actions_model.ActionRunner{Name: "ephemeral", AgentLabels: []string{"ubuntu-latest"}, Ephemeral: true}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(ctx, runner))

	task, ok, err := PickTask(ctx, runner)
	require.NoError(t, err)
	assert.Nil(t, task)
	assert.False(t, ok)

	failed := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
	assert.Equal(t, actions_model.StatusFailure, failed.Status)
	assert.Zero(t, failed.TaskID)
	unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
}
