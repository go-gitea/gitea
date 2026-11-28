// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func environmentJobPayload(environment string) []byte {
	return fmt.Appendf(nil, "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: %q\n    steps:\n      - run: echo hi\n", environment)
}

func TestResolveJobEnvironment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})

	_, _, err := GetOrCreateEnvironment(ctx, repo, "production", []string{"main"})
	require.NoError(t, err)

	resolve := func(environment string, run *actions_model.ActionRun) (*actions_model.ActionEnvironment, error) {
		job := &actions_model.ActionRunJob{RepoID: repo.ID, JobID: "deploy", WorkflowPayload: environmentJobPayload(environment), Run: run}
		env, denied, err := resolveJobEnvironment(ctx, job)
		require.NoError(t, err)
		return env, denied
	}
	push := func(ref string) *actions_model.ActionRun {
		return &actions_model.ActionRun{RepoID: repo.ID, Ref: ref, TriggerEvent: "push"}
	}

	t.Run("allowed ref", func(t *testing.T) {
		env, denied := resolve("PRODUCTION", push("refs/heads/main"))
		require.NotNil(t, env)
		assert.NoError(t, denied)
	})
	t.Run("ref refused by the policy", func(t *testing.T) {
		_, denied := resolve("production", push("refs/heads/feature"))
		assert.ErrorContains(t, denied, "not allowed")
	})
	t.Run("pull_request_target is judged by the ref its workflow runs from", func(t *testing.T) {
		run := &actions_model.ActionRun{
			RepoID: repo.ID, Ref: "refs/pull/1/head", Event: "pull_request", TriggerEvent: "pull_request_target", WorkflowRef: "refs/heads/main",
			EventPayload: `{"pull_request":{"base":{"ref":"release/1.0"},"head":{"ref":"feature"}}}`,
		}
		_, denied := resolve("production", run)
		assert.NoError(t, denied)
	})
	t.Run("invalid name", func(t *testing.T) {
		env, denied := resolve(strings.Repeat("x", actions_model.EnvironmentNameMaxLength+1), push("refs/heads/main"))
		assert.Nil(t, env)
		assert.ErrorContains(t, denied, "invalid")
	})
	t.Run("missing environment is created without a policy", func(t *testing.T) {
		env, denied := resolve("staging", push("refs/heads/feature"))
		require.NotNil(t, env)
		assert.NoError(t, denied)
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "staging"})
	})
	t.Run("untrusted fork run", func(t *testing.T) {
		run := push("refs/pull/1/head")
		run.IsForkPullRequest, run.TriggerEvent = true, "pull_request"
		for _, name := range []string{"production", "from-fork"} {
			env, denied := resolve(name, run)
			assert.Nil(t, env)
			assert.NoError(t, denied)
		}
		unittest.AssertNotExistsBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "from-fork"})
	})
}

func TestDeniedJobKeepsEphemeralRunner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(int64) error { return nil })()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	_, _, err := GetOrCreateEnvironment(ctx, repo, "production", []string{"main"})
	require.NoError(t, err)
	run := &actions_model.ActionRun{
		Title: "denied-run", RepoID: repo.ID, OwnerID: repo.OwnerID, WorkflowID: "test.yaml", TriggerUserID: 2,
		Ref: "refs/heads/feature", CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event: "push", TriggerEvent: "push", Status: actions_model.StatusWaiting,
	}
	require.NoError(t, db.Insert(ctx, run))
	runner := &actions_model.ActionRunner{Name: "ephemeral", AgentLabels: []string{"ubuntu-latest"}, Ephemeral: true}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(ctx, runner))

	// an invalid name of any length must still fail the job, or it would be claimed and released forever
	for _, environment := range []string{"production", strings.Repeat("x", actions_model.MaxJobSummarySize)} {
		job := &actions_model.ActionRunJob{
			RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
			Name: "deploy", Attempt: 1, JobID: "deploy", Status: actions_model.StatusWaiting,
			RunsOn: []string{"ubuntu-latest"}, WorkflowPayload: environmentJobPayload(environment),
		}
		require.NoError(t, db.Insert(ctx, job))

		task, ok, err := PickTask(ctx, runner)
		require.NoError(t, err)
		assert.Nil(t, task)
		assert.False(t, ok)

		failed := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusFailure, failed.Status)
		assert.Zero(t, failed.TaskID)
	}
	unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
}
