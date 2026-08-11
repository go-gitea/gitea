// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryPickTaskThrottled(t *testing.T) {
	sem := taskPickLimiter()

	// Saturate every assignment slot so the next attempt must be throttled.
	for range cap(sem) {
		sem <- struct{}{}
	}
	defer func() {
		for range cap(sem) {
			<-sem
		}
	}()

	// No DB access happens on the throttled path, so this is safe without fixtures.
	task, ok, throttled, err := TryPickTask(t.Context(), &actions_model.ActionRunner{})
	require.NoError(t, err)
	assert.Nil(t, task)
	assert.False(t, ok)
	assert.True(t, throttled)
}

// TestReleaseTaskForRunnerCleanup verifies the cleanup used by PickTask releases a claimed task through a
// fresh context. PickTask reaches this path when the request context is already canceled, and on
// PostgreSQL/MySQL a DB transaction on a canceled context fails immediately; reusing it would strand the
// claimed job in running state, so the cleanup must not use the request context.
func TestReleaseTaskForRunnerCleanup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	run := &actions_model.ActionRun{
		Title: "cleanup-run", RepoID: 1, OwnerID: 2, WorkflowID: "test.yaml",
		TriggerUserID: 2, Ref: "refs/heads/main",
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Event: "push", TriggerEvent: "push",
		Status: actions_model.StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	job := &actions_model.ActionRunJob{
		RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
		Name: "cleanup-job", Attempt: 1, JobID: "cleanup-job", Status: actions_model.StatusWaiting,
		RunsOn:          []string{"ubuntu-latest"},
		WorkflowPayload: []byte("on: push\njobs:\n  cleanup-job:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"),
	}
	require.NoError(t, db.Insert(t.Context(), job))
	runner := &actions_model.ActionRunner{Name: "cleanup-runner", AgentLabels: []string{"ubuntu-latest"}}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))

	task, ok, err := actions_model.CreateTaskForRunner(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, actions_model.StatusRunning, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Status)

	// the cleanup helper uses its own context, so the claimed job is returned to the waiting queue
	releaseTaskForRunnerCleanup(task)
	released := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
	assert.Equal(t, actions_model.StatusWaiting, released.Status)
	assert.Zero(t, released.TaskID)
	unittest.AssertNotExistsBean(t, &actions_model.ActionTask{ID: task.ID})
}

func TestGenerateTaskContextReusableEventCompatibility(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	require.NoError(t, repo.LoadOwner(ctx))
	actor := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	run := &actions_model.ActionRun{
		RepoID:        repo.ID,
		Repo:          repo,
		OwnerID:       repo.OwnerID,
		TriggerUserID: actor.ID,
		TriggerUser:   actor,
		WorkflowID:    "caller.yml",
		Index:         99602,
		Ref:           "refs/heads/main",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))

	caller := &actions_model.ActionRunJob{
		RunID:            run.ID,
		RepoID:           repo.ID,
		OwnerID:          repo.OwnerID,
		CommitSHA:        run.CommitSHA,
		Name:             "caller",
		JobID:            "caller",
		Attempt:          1,
		Status:           actions_model.StatusRunning,
		CallPayload:      "{}",
		IsReusableCaller: true,
		IsExpanded:       true,
	}
	require.NoError(t, db.Insert(ctx, caller))

	child := &actions_model.ActionRunJob{
		RunID:       run.ID,
		Run:         run,
		RepoID:      repo.ID,
		OwnerID:     repo.OwnerID,
		CommitSHA:   run.CommitSHA,
		Name:        "child",
		JobID:       "child",
		Attempt:     1,
		Status:      actions_model.StatusRunning,
		ParentJobID: caller.ID,
	}
	require.NoError(t, db.Insert(ctx, child))

	task := &actions_model.ActionTask{
		ID:    99602,
		JobID: child.ID,
		Job:   child,
		Token: "task-token",
	}

	topLevelJob := &actions_model.ActionRunJob{
		RunID:     run.ID,
		Run:       run,
		RepoID:    repo.ID,
		OwnerID:   repo.OwnerID,
		CommitSHA: run.CommitSHA,
		Name:      "top-level",
		JobID:     "top-level",
		Attempt:   1,
		Status:    actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, topLevelJob))

	topLevelTask := &actions_model.ActionTask{
		ID:    99603,
		JobID: topLevelJob.ID,
		Job:   topLevelJob,
		Token: "task-token",
	}

	t.Run("legacy runner gets workflow_call compatibility event", func(t *testing.T) {
		runner := &actions_model.ActionRunner{}

		taskContext, err := generateTaskContext(ctx, task, runner)
		require.NoError(t, err)

		assert.Equal(t, "workflow_call", taskContext.Fields["event_name"].GetStringValue())
	})

	t.Run("capable runner gets original event", func(t *testing.T) {
		runner := &actions_model.ActionRunner{
			HasWorkflowCallOriginalEventSupport: true,
		}

		taskContext, err := generateTaskContext(ctx, task, runner)
		require.NoError(t, err)

		assert.Equal(t, "push", taskContext.Fields["event_name"].GetStringValue())
	})

	t.Run("legacy runner keeps original event for top-level job", func(t *testing.T) {
		runner := &actions_model.ActionRunner{}

		taskContext, err := generateTaskContext(ctx, topLevelTask, runner)
		require.NoError(t, err)

		assert.Equal(t, "push", taskContext.Fields["event_name"].GetStringValue())
	})
}
