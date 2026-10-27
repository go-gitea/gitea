// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"
	"testing"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"xorm.io/xorm/contexts"
)

type runnerClaimHookKey struct{}

type runnerClaimHook struct {
	before func(*contexts.ContextHook) error
	after  func(*contexts.ContextHook) error
}

func (h *runnerClaimHook) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	if c.Ctx.Value(runnerClaimHookKey{}) == h && h.before != nil {
		return c.Ctx, h.before(c)
	}
	return c.Ctx, nil
}

func (h *runnerClaimHook) AfterProcess(c *contexts.ContextHook) error {
	if c.Ctx.Value(runnerClaimHookKey{}) == h && h.after != nil {
		return h.after(c)
	}
	return nil
}

func runnerClaimHookContext(t *testing.T, h *runnerClaimHook) context.Context {
	t.Helper()
	unittest.GetXORMEngine().AddHook(h)
	return context.WithValue(t.Context(), runnerClaimHookKey{}, h)
}

func isRunnerClaimLock(sql string) bool {
	return strings.Contains(sql, "action_runner") &&
		(strings.Contains(sql, "FOR UPDATE") || strings.Contains(sql, "is_disabled = is_disabled"))
}

func newRunnerClaimJob(t *testing.T) (*ActionRunner, *ActionRunJob) {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	run := &ActionRun{
		Title: "runner-barrier", RepoID: 1, OwnerID: 2, WorkflowID: "test.yaml", Index: 9910,
		TriggerUserID: 2, Ref: "refs/heads/main", Event: "push", TriggerEvent: "push",
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Status: StatusWaiting,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	job := &ActionRunJob{
		RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
		Name: "barrier", Attempt: 1, JobID: "barrier", Status: StatusWaiting,
		RunsOn:          []string{"ubuntu-latest"},
		WorkflowPayload: []byte("on: push\njobs:\n  barrier:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n"),
	}
	require.NoError(t, db.Insert(t.Context(), job))
	runner := &ActionRunner{UUID: "barrier-runner-uuid", Name: "barrier-runner", AgentLabels: []string{"ubuntu-latest"}}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))
	return runner, job
}

func assertRunnerJobUnclaimed(t *testing.T, runner *ActionRunner, job *ActionRunJob) {
	t.Helper()
	actual := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	assert.Equal(t, StatusWaiting, actual.Status)
	assert.Zero(t, actual.TaskID)
	unittest.AssertNotExistsBean(t, &ActionTask{RunnerID: runner.ID})
	unittest.AssertNotExistsBean(t, &ActionTaskStep{RepoID: job.RepoID})
}

func TestRunnerDisabledBarrier(t *testing.T) {
	t.Run("stale enabled fetch cannot allocate", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		maintenance := *runner
		require.NoError(t, SetRunnerDisabled(t.Context(), &maintenance, true))
		assert.False(t, runner.IsDisabled)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, task)
		assertRunnerJobUnclaimed(t, runner, job)
	})

	t.Run("stale same value cannot skip disable or enable", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		runner.IsDisabled = true
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
		actual, err := GetRunnerByID(t.Context(), runner.ID)
		require.NoError(t, err)
		assert.True(t, actual.IsDisabled)
		runner.IsDisabled = false
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, false))
		actual, err = GetRunnerByID(t.Context(), runner.ID)
		require.NoError(t, err)
		assert.False(t, actual.IsDisabled)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, runner.ID, task.RunnerID)
	})

	t.Run("repeat disable still locks without bumping version", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
		version, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		locks := 0
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if isRunnerClaimLock(c.SQL) {
				locks++
			}
			return nil
		}})
		require.NoError(t, SetRunnerDisabled(ctx, runner, true))
		assert.Equal(t, 1, locks)
		after, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, version, after)
	})

	t.Run("missing and deleted runner cannot allocate", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		_, err := db.DeleteByID[ActionRunner](t.Context(), runner.ID)
		require.NoError(t, err)
		err = SetRunnerDisabled(t.Context(), runner, true)
		require.ErrorIs(t, err, util.ErrNotExist)
		assert.False(t, runner.IsDisabled)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.ErrorIs(t, err, util.ErrNotExist)
		assert.False(t, ok)
		assert.Nil(t, task)
		assertRunnerJobUnclaimed(t, runner, job)
	})

	t.Run("disabled stops scanning remaining candidates", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		second := *job
		second.ID = 0
		second.JobID = "barrier-second"
		second.WorkflowPayload = []byte(strings.ReplaceAll(string(job.WorkflowPayload), "barrier:", "barrier-second:"))
		require.NoError(t, db.Insert(t.Context(), &second))
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
		runner.IsDisabled = false
		scans := 0
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if isRunnerClaimLock(c.SQL) {
				scans++
			}
			return nil
		}})
		task, ok, err := CreateTaskForRunner(ctx, runner)
		require.NoError(t, err)
		assert.Nil(t, task)
		assert.False(t, ok)
		assert.Equal(t, 1, scans)
		assertRunnerJobUnclaimed(t, runner, job)
	})
}

func TestRunnerBarrierDatabaseFailures(t *testing.T) {
	injected := errors.New("interrupted runner maintenance transaction")
	for _, tc := range []struct {
		name  string
		match func(string) bool
	}{
		{"lock", isRunnerClaimLock},
		{"read", func(sql string) bool {
			return strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, "action_runner")
		}},
		{"update", func(sql string) bool {
			return strings.HasPrefix(sql, "UPDATE") && strings.Contains(sql, "action_runner") && !isRunnerClaimLock(sql)
		}},
		{"task version", func(sql string) bool { return strings.HasPrefix(sql, "UPDATE action_tasks_version") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, _ := newRunnerClaimJob(t)
			ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
				if tc.match(c.SQL) {
					return injected
				}
				return nil
			}})
			require.ErrorIs(t, SetRunnerDisabled(ctx, runner, true), injected)
			assert.False(t, runner.IsDisabled)
			actual, err := GetRunnerByID(t.Context(), runner.ID)
			require.NoError(t, err)
			assert.False(t, actual.IsDisabled)
			require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
		})
	}

	t.Run("claim read failure rolls back", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if strings.HasPrefix(c.SQL, "SELECT") && strings.Contains(c.SQL, "action_runner") {
				return injected
			}
			return nil
		}})
		task, ok, err := CreateTaskForRunner(ctx, runner)
		require.ErrorIs(t, err, injected)
		assert.Nil(t, task)
		assert.False(t, ok)
		assertRunnerJobUnclaimed(t, runner, job)
	})

	t.Run("cancelled context is not successful maintenance", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, SetRunnerDisabled(ctx, runner, true), context.Canceled)
		assert.False(t, runner.IsDisabled)
		assertRunnerJobUnclaimed(t, runner, job)
	})
}

func TestRunnerBarrierKeepsOptimisticJobClaim(t *testing.T) {
	runner, job := newRunnerClaimJob(t)
	task, ok, err := CreateTaskForRunner(t.Context(), runner)
	require.NoError(t, err)
	require.True(t, ok)
	loser := *runner
	loser.ID = 0
	loser.UUID = "barrier-second-runner"
	loser.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), &loser))
	duplicate, ok, err := claimJobForRunner(t.Context(), &loser, job)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, duplicate)
	unittest.AssertCount(t, &ActionTask{}, 1)
	unittest.AssertCount(t, &ActionTaskStep{}, 1)
	actual := unittest.AssertExistsAndLoadBean(t, &ActionRunJob{ID: job.ID})
	assert.Equal(t, task.ID, actual.TaskID)
}

func TestRunnerBulkBarrier(t *testing.T) {
	t.Run("all locks precede versions in ascending ID order", func(t *testing.T) {
		first, _ := newRunnerClaimJob(t)
		second := *first
		second.ID, second.UUID = 0, "barrier-second-runner"
		second.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), &second))
		var locked []any
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if isRunnerClaimLock(c.SQL) {
				locked = append(locked, c.Args[0])
			}
			if strings.HasPrefix(c.SQL, "UPDATE action_tasks_version") {
				assert.Equal(t, []any{first.ID, second.ID}, locked)
			}
			return nil
		}})
		runners := []*ActionRunner{&second, first}
		require.NoError(t, SetRunnersDisabled(ctx, runners, true))
		assert.Equal(t, []any{first.ID, second.ID}, locked)
		assert.Equal(t, second.ID, runners[0].ID)
		assert.True(t, first.IsDisabled)
		assert.True(t, second.IsDisabled)
		require.NoError(t, SetRunnersDisabled(t.Context(), runners, false))
		assert.False(t, first.IsDisabled)
		assert.False(t, second.IsDisabled)
		require.NoError(t, SetRunnersDisabled(t.Context(), nil, true))
	})

	t.Run("second update failure rolls back the entire batch", func(t *testing.T) {
		first, _ := newRunnerClaimJob(t)
		second := *first
		second.ID, second.UUID = 0, "barrier-second-runner"
		second.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), &second))
		version, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		injected := errors.New("second runner write interrupted")
		updates := 0
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if strings.HasPrefix(c.SQL, "UPDATE") && strings.Contains(c.SQL, "action_runner") && !isRunnerClaimLock(c.SQL) {
				updates++
				if updates == 2 {
					return injected
				}
			}
			return nil
		}})
		require.ErrorIs(t, SetRunnersDisabled(ctx, []*ActionRunner{&second, first}, true), injected)
		assert.False(t, first.IsDisabled)
		assert.False(t, second.IsDisabled)
		for _, id := range []int64{first.ID, second.ID} {
			actual, err := GetRunnerByID(t.Context(), id)
			require.NoError(t, err)
			assert.False(t, actual.IsDisabled)
		}
		after, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, version, after)
	})
}

func TestDisabledRunnerCanFinishCancellingTask(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	task, _ := newRunningTaskForCancelling(t, "disabled-cancelling", true)
	runner, err := GetRunnerByID(t.Context(), task.RunnerID)
	require.NoError(t, err)
	require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
	require.NoError(t, StopTask(t.Context(), task.ID, StatusCancelling))
	updated, err := UpdateTaskByState(t.Context(), runner.ID, &runnerv1.TaskState{
		Id: task.ID, Result: runnerv1.Result_RESULT_SUCCESS, StoppedAt: timestamppb.Now(),
	})
	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, updated.Status)
	assert.Positive(t, int64(updated.Stopped))
}

func TestRunnerClaimRollbackPaths(t *testing.T) {
	injected := errors.New("interrupted claim write")
	for _, tc := range []struct{ name, verb, table string }{
		{"queue query", "SELECT", "action_run_job"},
		{"job attributes", "SELECT", "action_run"},
		{"task insert", "INSERT", "action_task"},
		{"log filename", "UPDATE", "action_task"},
		{"step insert", "INSERT", "action_task_step"},
		{"job update", "UPDATE", "action_run_job"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, job := newRunnerClaimJob(t)
			ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
				sql := strings.NewReplacer("`", "", "\"", "").Replace(c.SQL)
				if strings.HasPrefix(sql, tc.verb+" ") && strings.Contains(sql, " "+tc.table+" ") {
					return injected
				}
				return nil
			}})
			task, ok, err := CreateTaskForRunner(ctx, runner)
			require.ErrorIs(t, err, injected)
			assert.Nil(t, task)
			assert.False(t, ok)
			assertRunnerJobUnclaimed(t, runner, job)
		})
	}

	t.Run("invalid workflow never persists partial allocation", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		job.WorkflowPayload = []byte("jobs: [")
		_, err := db.GetEngine(t.Context()).ID(job.ID).Cols("workflow_payload").Update(job)
		require.NoError(t, err)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.Error(t, err)
		assert.Nil(t, task)
		assert.False(t, ok)
		assertRunnerJobUnclaimed(t, runner, job)
	})

	t.Run("workflow without steps", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		job.WorkflowPayload = []byte("on: push\njobs:\n  barrier:\n    runs-on: ubuntu-latest\n    steps: []\n")
		_, err := db.GetEngine(t.Context()).ID(job.ID).Cols("workflow_payload").Update(job)
		require.NoError(t, err)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Empty(t, task.Steps)
	})

	t.Run("transactional caller is rejected", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		require.NoError(t, db.WithTx(t.Context(), func(ctx context.Context) error {
			task, ok, err := CreateTaskForRunner(ctx, runner)
			require.ErrorContains(t, err, "must not be called within a database transaction")
			assert.Nil(t, task)
			assert.False(t, ok)
			return nil
		}))
		assertRunnerJobUnclaimed(t, runner, job)
	})
}

func TestRunnerClaimScopeAndNoMatchingWork(t *testing.T) {
	for _, repoScoped := range []bool{true, false} {
		name := "owner runner"
		if repoScoped {
			name = "repository runner"
		}
		t.Run(name, func(t *testing.T) {
			runner, job := newRunnerClaimJob(t)
			if repoScoped {
				runner.RepoID = job.RepoID
			} else {
				runner.OwnerID = job.OwnerID
				require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{RepoID: job.RepoID, Type: unit.TypeActions}))
			}
			require.NoError(t, UpdateRunner(t.Context(), runner, "repo_id", "owner_id"))
			task, ok, err := CreateTaskForRunner(t.Context(), runner)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, job.ID, task.JobID)
		})
	}
	t.Run("no matching labels returns without allocation", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		runner.AgentLabels = []string{"windows"}
		require.NoError(t, UpdateRunner(t.Context(), runner, "agent_labels"))
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		assert.Nil(t, task)
		assert.False(t, ok)
		assertRunnerJobUnclaimed(t, runner, job)
	})
}

func TestEphemeralRunnerClaimLifecycle(t *testing.T) {
	t.Run("MSSQL one-shot lookup uses NOWAIT and propagates lock errors", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		runner.Ephemeral = true
		require.NoError(t, UpdateRunner(t.Context(), runner, "ephemeral"))
		oldType := setting.Database.Type
		setting.Database.Type = "mssql"
		t.Cleanup(func() { setting.Database.Type = oldType })
		injected := errors.New("MSSQL task lookup lock conflict")
		lookups := 0
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			if strings.Contains(c.SQL, "WITH (NOWAIT)") {
				lookups++
				assert.Equal(t, "SELECT TOP (1) 1 FROM action_task WITH (NOWAIT) WHERE runner_id = ?", c.SQL)
				assert.Equal(t, []any{runner.ID}, c.Args)
				return injected
			}
			return nil
		}})
		task, ok, err := CreateTaskForRunner(ctx, runner)
		require.ErrorIs(t, err, injected)
		assert.Equal(t, 1, lookups)
		assert.False(t, ok)
		assert.Nil(t, task)
		assertRunnerJobUnclaimed(t, runner, job)
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
	})

	t.Run("a stored completion still consumes the one-shot runner", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		runner.Ephemeral = true
		require.NoError(t, UpdateRunner(t.Context(), runner, "ephemeral"))
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		task.Status = StatusSuccess
		_, err = db.GetEngine(t.Context()).ID(task.ID).Cols("status").Update(task)
		require.NoError(t, err)
		job := *task.Job
		job.ID, job.TaskID, job.Status = 0, 0, StatusWaiting
		require.NoError(t, db.Insert(t.Context(), &job))
		runner.Ephemeral = false
		next, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, next)
		unittest.AssertCount(t, &ActionTask{RunnerID: runner.ID}, 1)
	})

	t.Run("failure reading one-shot usage does not allocate", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		runner.Ephemeral = true
		require.NoError(t, UpdateRunner(t.Context(), runner, "ephemeral"))
		injected := errors.New("ephemeral usage read interrupted")
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			sql := strings.NewReplacer("`", "", "\"", "").Replace(c.SQL)
			if strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, " FROM action_task ") {
				return injected
			}
			return nil
		}})
		task, ok, err := CreateTaskForRunner(ctx, runner)
		require.ErrorIs(t, err, injected)
		assert.False(t, ok)
		assert.Nil(t, task)
		assertRunnerJobUnclaimed(t, runner, job)
	})

	t.Run("released task cannot retire its reusable registration", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		runner.Ephemeral = true
		require.NoError(t, UpdateRunner(t.Context(), runner, "ephemeral"))
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, ReleaseTaskForRunner(t.Context(), task))
		task.Status = StatusCancelled
		require.NoError(t, UpdateTask(t.Context(), task, "status"))
		unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: runner.ID})
		next, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		assert.NotEqual(t, task.ID, next.ID)
	})

	t.Run("implicit columns preserve task metadata updates", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		task, ok, err := CreateTaskForRunner(t.Context(), runner)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, UpdateTask(t.Context(), &ActionTask{ID: task.ID, LogLength: 1}))
		actual := unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: task.ID})
		assert.EqualValues(t, 1, actual.LogLength)
		assert.Equal(t, StatusRunning, actual.Status)
	})
}
