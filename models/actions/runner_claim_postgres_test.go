// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/contexts"
	"xorm.io/xorm/names"
)

type runnerClaimRoleKey struct{}

type runnerClaimResult struct {
	task *ActionTask
	ok   bool
	err  error
}

func runnerClaimPostgres(t *testing.T) *xorm.Engine {
	t.Helper()
	dsn := os.Getenv("GITEA_TEST_RUNNER_POSTGRES")
	if dsn == "" {
		t.Skip("GITEA_TEST_RUNNER_POSTGRES is required for PostgreSQL lock tests")
	}
	oldEngine := unittest.GetXORMEngine()
	oldType := setting.Database.Type
	engine, err := xorm.NewEngine("postgres", dsn)
	require.NoError(t, err)
	engine.SetMapper(names.GonicMapper{})
	engine.SetMaxOpenConns(5)
	db.SetDefaultEngine(context.Background(), engine)
	setting.Database.Type = "postgres"
	fixtures := unittest.FixturesOptions{
		Dir:   filepath.Join(setting.GetGiteaTestSourceRoot(), "models", "fixtures"),
		Files: []string{"action_runner_token.yml", "action_run.yml", "repository.yml", "user.yml"},
	}
	t.Cleanup(func() {
		db.SetDefaultEngine(context.Background(), oldEngine)
		setting.Database.Type = oldType
		assert.NoError(t, unittest.InitFixtures(fixtures))
		assert.NoError(t, engine.Close())
	})
	require.NoError(t, db.SyncAllTables())
	require.NoError(t, unittest.InitFixtures(fixtures))
	metadata, err := engine.QueryString("SELECT current_setting('server_version') AS version, current_setting('default_transaction_isolation') AS isolation")
	require.NoError(t, err)
	t.Logf("本地 PostgreSQL 后端与默认隔离级别: %v", metadata)
	return engine
}

func runnerClaimAsync(ctx context.Context, runner *ActionRunner, job *ActionRunJob, disable bool) (<-chan int, <-chan runnerClaimResult) {
	pid := make(chan int, 1)
	done := make(chan runnerClaimResult, 1)
	go func() {
		var result runnerClaimResult
		result.err = db.WithTx(ctx, func(ctx context.Context) error {
			var backendPID int
			if _, err := db.GetEngine(ctx).Context(ctx).SQL("SELECT pg_backend_pid()").Get(&backendPID); err != nil {
				return err
			}
			pid <- backendPID
			if disable {
				return SetRunnerDisabled(ctx, runner, true)
			}
			result.task, result.ok, result.err = claimJobForRunner(ctx, runner, job)
			return result.err
		})
		done <- result
	}()
	return pid, done
}

func awaitRunnerClaim[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(10 * time.Second):
		t.Fatal("runner transaction did not reach the expected synchronization point")
		var zero T
		return zero
	}
}

func awaitRunnerBlocked(t *testing.T, engine *xorm.Engine, pid int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked bool
		err := engine.DB().QueryRow("SELECT cardinality(pg_blocking_pids($1)) > 0", pid).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 5*time.Millisecond)
	var blockers string
	require.NoError(t, engine.DB().QueryRow("SELECT pg_blocking_pids($1)::text", pid).Scan(&blockers))
	assert.NotEqual(t, "{}", blockers)
	t.Logf("本地 PostgreSQL 实际锁等待: pid=%d blockers=%s, SQL=pg_blocking_pids", pid, blockers)
}

func TestRunnerBarrierPostgres(t *testing.T) {
	engine := runnerClaimPostgres(t)
	t.Run("assignment and maintenance query overhead", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		locks, runnerWrites, statements := 0, 0, 0
		ctx := runnerClaimHookContext(t, &runnerClaimHook{before: func(c *contexts.ContextHook) error {
			statements++
			if isRunnerClaimLock(c.SQL) {
				locks++
			}
			if strings.HasPrefix(c.SQL, "UPDATE") && strings.Contains(c.SQL, "action_runner") {
				runnerWrites++
			}
			return nil
		}})
		task, ok, err := CreateTaskForRunner(ctx, runner)
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, task)
		assert.Equal(t, 1, locks)
		assert.Zero(t, runnerWrites)
		t.Logf("PG实际单次成功分配: Runner锁查询=%d Runner写SQL=%d 总SQL=%d", locks, runnerWrites, statements)
		locks, runnerWrites, statements = 0, 0, 0
		task, ok, err = CreateTaskForRunner(ctx, runner)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, task)
		assert.Zero(t, locks)
		assert.Zero(t, runnerWrites)
		t.Logf("PG实际空候选查询: Runner锁查询=%d Runner写SQL=%d 总SQL=%d", locks, runnerWrites, statements)
		require.NoError(t, SetRunnerDisabled(t.Context(), runner, true))
		locks, runnerWrites, statements = 0, 0, 0
		require.NoError(t, SetRunnerDisabled(ctx, runner, true))
		assert.Equal(t, 1, locks)
		assert.Zero(t, runnerWrites)
		t.Logf("PG实际重复禁用: Runner锁查询=%d Runner写SQL=%d 总SQL=%d", locks, runnerWrites, statements)
	})
	t.Run("competing fetch retries the next waiting job", func(t *testing.T) {
		first, job := newRunnerClaimJob(t)
		second := *first
		second.ID, second.UUID = 0, "barrier-second-runner"
		second.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), &second))
		nextJob := *job
		nextJob.ID, nextJob.JobID = 0, "barrier-second"
		nextJob.WorkflowPayload = []byte(strings.ReplaceAll(string(job.WorkflowPayload), "barrier:", "barrier-second:"))
		require.NoError(t, db.Insert(t.Context(), &nextJob))
		makeCtx := func(held, release chan struct{}) context.Context {
			stalled := false
			return runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
				sql := strings.NewReplacer("`", "", "\"", "", " ", "").Replace(c.SQL)
				if !stalled && strings.Contains(sql, "FROMaction_run_job") && strings.Contains(sql, "task_id=") && strings.HasPrefix(sql, "SELECT") {
					stalled = true
					held <- struct{}{}
					<-release
				}
				return nil
			}})
		}
		firstHeld, secondHeld := make(chan struct{}, 1), make(chan struct{}, 1)
		firstRelease, secondRelease := make(chan struct{}, 1), make(chan struct{}, 1)
		t.Cleanup(func() {
			select {
			case firstRelease <- struct{}{}:
			default:
			}
			select {
			case secondRelease <- struct{}{}:
			default:
			}
		})
		firstCtx, secondCtx := makeCtx(firstHeld, firstRelease), makeCtx(secondHeld, secondRelease)
		firstDone, secondDone := make(chan runnerClaimResult, 1), make(chan runnerClaimResult, 1)
		fetch := func(ctx context.Context, runner *ActionRunner, done chan runnerClaimResult) {
			task, ok, err := CreateTaskForRunner(ctx, runner)
			done <- runnerClaimResult{task: task, ok: ok, err: err}
		}
		go fetch(firstCtx, first, firstDone)
		awaitRunnerClaim(t, firstHeld)
		go fetch(secondCtx, &second, secondDone)
		awaitRunnerClaim(t, secondHeld)
		firstRelease <- struct{}{}
		firstResult := awaitRunnerClaim(t, firstDone)
		require.NoError(t, firstResult.err)
		require.True(t, firstResult.ok)
		secondRelease <- struct{}{}
		secondResult := awaitRunnerClaim(t, secondDone)
		require.NoError(t, secondResult.err)
		require.True(t, secondResult.ok)
		assert.Equal(t, job.ID, firstResult.task.JobID)
		assert.Equal(t, nextJob.ID, secondResult.task.JobID)
		unittest.AssertCount(t, &ActionTask{}, 2)
		unittest.AssertCount(t, &ActionTaskStep{}, 2)
	})
	t.Run("commit failure rolls back state and version", func(t *testing.T) {
		runner, _ := newRunnerClaimJob(t)
		version, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		_, err = engine.Exec(`CREATE FUNCTION runner_barrier_reject_disabled() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'local commit rejection'; END $$`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := engine.Exec("DROP FUNCTION runner_barrier_reject_disabled() CASCADE")
			assert.NoError(t, err)
		})
		_, err = engine.Exec(`CREATE CONSTRAINT TRIGGER runner_barrier_commit_failure AFTER UPDATE ON action_runner DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.is_disabled) EXECUTE FUNCTION runner_barrier_reject_disabled()`)
		require.NoError(t, err)
		require.ErrorContains(t, SetRunnerDisabled(t.Context(), runner, true), "local commit rejection")
		assert.False(t, runner.IsDisabled)
		actual, err := GetRunnerByID(t.Context(), runner.ID)
		require.NoError(t, err)
		assert.False(t, actual.IsDisabled)
		after, err := GetTasksVersionByScope(t.Context(), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, version, after)
	})
	for _, disableFirst := range []bool{true, false} {
		name := "claim commits before maintenance"
		if disableFirst {
			name = "maintenance blocks stale claim"
		}
		t.Run(name, func(t *testing.T) {
			runner, job := newRunnerClaimJob(t)
			maintenance := *runner
			held, release := make(chan struct{}, 1), make(chan struct{}, 1)
			ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
				if c.Ctx.Value(runnerClaimRoleKey{}) == "first" && isRunnerClaimLock(c.SQL) && c.Err == nil {
					held <- struct{}{}
					<-release
				}
				return nil
			}})
			t.Cleanup(func() {
				select {
				case release <- struct{}{}:
				default:
				}
			})
			_, first := runnerClaimAsync(context.WithValue(ctx, runnerClaimRoleKey{}, "first"), &maintenance, job, disableFirst)
			awaitRunnerClaim(t, held)
			secondPID, second := runnerClaimAsync(ctx, runner, job, !disableFirst)
			awaitRunnerBlocked(t, engine, awaitRunnerClaim(t, secondPID))
			release <- struct{}{}
			firstResult := awaitRunnerClaim(t, first)
			secondResult := awaitRunnerClaim(t, second)
			require.NoError(t, firstResult.err)
			if disableFirst {
				require.ErrorIs(t, secondResult.err, errRunnerDisabled)
				assertRunnerJobUnclaimed(t, runner, job)
			} else {
				require.NoError(t, secondResult.err)
				require.True(t, firstResult.ok)
				unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: firstResult.task.ID, RunnerID: runner.ID})
			}
			actual, err := GetRunnerByID(t.Context(), runner.ID)
			require.NoError(t, err)
			assert.True(t, actual.IsDisabled)
		})
	}

	t.Run("another runner progresses while the first is locked", func(t *testing.T) {
		first, job := newRunnerClaimJob(t)
		second := *first
		second.ID, second.UUID = 0, "barrier-second-runner"
		second.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), &second))
		held, release := make(chan struct{}, 1), make(chan struct{}, 1)
		ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
			if c.Ctx.Value(runnerClaimRoleKey{}) == "holder" && isRunnerClaimLock(c.SQL) && c.Err == nil {
				held <- struct{}{}
				<-release
			}
			return nil
		}})
		t.Cleanup(func() {
			select {
			case release <- struct{}{}:
			default:
			}
		})
		_, holder := runnerClaimAsync(context.WithValue(ctx, runnerClaimRoleKey{}, "holder"), first, job, false)
		awaitRunnerClaim(t, held)
		_, other := runnerClaimAsync(ctx, &second, job, false)
		result := awaitRunnerClaim(t, other)
		require.NoError(t, result.err)
		require.True(t, result.ok)
		assert.Equal(t, second.ID, result.task.RunnerID)
		release <- struct{}{}
		firstResult := awaitRunnerClaim(t, holder)
		require.NoError(t, firstResult.err)
		assert.False(t, firstResult.ok)
	})

	t.Run("cancelled lock waiter does not commit", func(t *testing.T) {
		runner, job := newRunnerClaimJob(t)
		held, release := make(chan struct{}, 1), make(chan struct{}, 1)
		ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
			if c.Ctx.Value(runnerClaimRoleKey{}) == "holder" && isRunnerClaimLock(c.SQL) && c.Err == nil {
				held <- struct{}{}
				<-release
			}
			return nil
		}})
		t.Cleanup(func() {
			select {
			case release <- struct{}{}:
			default:
			}
		})
		maintenance := *runner
		_, holder := runnerClaimAsync(context.WithValue(ctx, runnerClaimRoleKey{}, "holder"), &maintenance, job, false)
		awaitRunnerClaim(t, held)
		waitCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		pid, waiter := runnerClaimAsync(waitCtx, runner, job, true)
		awaitRunnerBlocked(t, engine, awaitRunnerClaim(t, pid))
		cancel()
		require.Error(t, awaitRunnerClaim(t, waiter).err)
		assert.False(t, runner.IsDisabled)
		release <- struct{}{}
		require.NoError(t, awaitRunnerClaim(t, holder).err)
		actual, err := GetRunnerByID(t.Context(), runner.ID)
		require.NoError(t, err)
		assert.False(t, actual.IsDisabled)
	})

	t.Run("bulk locks precede shared versions", func(t *testing.T) {
		first, _ := newRunnerClaimJob(t)
		second := *first
		second.ID, second.UUID = 0, "barrier-second-runner"
		second.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), &second))
		held, release := make(chan struct{}, 1), make(chan struct{}, 1)
		ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
			if c.Ctx.Value(runnerClaimRoleKey{}) == "single" && isRunnerClaimLock(c.SQL) && c.Err == nil {
				held <- struct{}{}
				<-release
			}
			return nil
		}})
		t.Cleanup(func() {
			select {
			case release <- struct{}{}:
			default:
			}
		})
		_, single := runnerClaimAsync(context.WithValue(ctx, runnerClaimRoleKey{}, "single"), &second, nil, true)
		awaitRunnerClaim(t, held)
		pid, done := make(chan int, 1), make(chan error, 1)
		go func() {
			done <- db.WithTx(ctx, func(ctx context.Context) error {
				var backendPID int
				if _, err := db.GetEngine(ctx).Context(ctx).SQL("SELECT pg_backend_pid()").Get(&backendPID); err != nil {
					return err
				}
				pid <- backendPID
				copySecond := second
				return SetRunnersDisabled(ctx, []*ActionRunner{&copySecond, first}, true)
			})
		}()
		awaitRunnerBlocked(t, engine, awaitRunnerClaim(t, pid))
		release <- struct{}{}
		require.NoError(t, awaitRunnerClaim(t, single).err)
		require.NoError(t, awaitRunnerClaim(t, done))
		for _, id := range []int64{first.ID, second.ID} {
			actual, err := GetRunnerByID(t.Context(), id)
			require.NoError(t, err)
			assert.True(t, actual.IsDisabled)
		}
	})

	for _, deleteSecond := range []bool{false, true} {
		name := "opposite bulk input order"
		if deleteSecond {
			name = "bulk delete follows the same order"
		}
		t.Run(name, func(t *testing.T) {
			first, _ := newRunnerClaimJob(t)
			second := *first
			second.ID, second.UUID = 0, "barrier-second-runner"
			second.GenerateAndFillToken()
			require.NoError(t, db.Insert(t.Context(), &second))
			held, release := make(chan struct{}, 1), make(chan struct{}, 1)
			firstLock := true
			ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
				if c.Ctx.Value(runnerClaimRoleKey{}) == "holder" && isRunnerClaimLock(c.SQL) && c.Err == nil && firstLock {
					firstLock = false
					held <- struct{}{}
					<-release
				}
				return nil
			}})
			t.Cleanup(func() {
				select {
				case release <- struct{}{}:
				default:
				}
			})
			firstDone := make(chan error, 1)
			firstTargets := []*ActionRunner{&second, first}
			go func() {
				firstDone <- SetRunnersDisabled(context.WithValue(ctx, runnerClaimRoleKey{}, "holder"), firstTargets, true)
			}()
			awaitRunnerClaim(t, held)
			pid, secondDone := make(chan int, 1), make(chan error, 1)
			go func() {
				secondDone <- db.WithTx(ctx, func(ctx context.Context) error {
					var backendPID int
					if _, err := db.GetEngine(ctx).Context(ctx).SQL("SELECT pg_backend_pid()").Get(&backendPID); err != nil {
						return err
					}
					pid <- backendPID
					targets, err := db.Find[ActionRunner](ctx, &FindRunnerOptions{IDs: []int64{second.ID, first.ID}, Sort: "oldest"})
					if err != nil {
						return err
					}
					if !deleteSecond {
						return SetRunnersDisabled(ctx, targets, false)
					}
					for _, target := range targets {
						if err := DeleteRunner(ctx, target.ID); err != nil {
							return err
						}
					}
					return nil
				})
			}()
			awaitRunnerBlocked(t, engine, awaitRunnerClaim(t, pid))
			release <- struct{}{}
			require.NoError(t, awaitRunnerClaim(t, firstDone))
			require.NoError(t, awaitRunnerClaim(t, secondDone))
			for _, id := range []int64{first.ID, second.ID} {
				if deleteSecond {
					unittest.AssertNotExistsBean(t, &ActionRunner{ID: id})
				} else {
					actual, err := GetRunnerByID(t.Context(), id)
					require.NoError(t, err)
					assert.False(t, actual.IsDisabled)
				}
			}
		})
	}
}

func TestRunnerEphemeralCancellationLockOrder(t *testing.T) {
	runnerClaimPostgres(t)
	for _, released := range []bool{false, true} {
		name := "existing task prevents a second allocation"
		if released {
			name = "released task cannot retire a reused runner"
		}
		t.Run(name, func(t *testing.T) {
			runner, firstJob := newRunnerClaimJob(t)
			runner.Ephemeral = true
			require.NoError(t, UpdateRunner(t.Context(), runner, "ephemeral"))
			attempt := &ActionRunAttempt{RepoID: firstJob.RepoID, RunID: firstJob.RunID, Attempt: 1, TriggerUserID: 2, Status: StatusRunning}
			require.NoError(t, db.Insert(t.Context(), attempt))
			run := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: firstJob.RunID})
			run.LatestAttemptID = attempt.ID
			require.NoError(t, UpdateRun(t.Context(), run, "latest_attempt_id"))
			firstJob.RunAttemptID = attempt.ID
			_, err := db.GetEngine(t.Context()).ID(firstJob.ID).Cols("run_attempt_id").Update(firstJob)
			require.NoError(t, err)
			secondJob := *firstJob
			secondJob.ID, secondJob.JobID = 0, "barrier-second"
			secondJob.WorkflowPayload = []byte(strings.ReplaceAll(string(firstJob.WorkflowPayload), "barrier:", "barrier-second:"))
			require.NoError(t, db.Insert(t.Context(), &secondJob))
			task, ok, err := CreateTaskForRunner(t.Context(), runner)
			require.NoError(t, err)
			require.True(t, ok)
			staleRead, attemptHeld, runnerHeld := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}, 1)
			resumeRead, resumeStop := make(chan struct{}, 1), make(chan struct{}, 1)
			readSignalled, attemptSignalled := false, false
			ctx := runnerClaimHookContext(t, &runnerClaimHook{after: func(c *contexts.ContextHook) error {
				if c.Err != nil {
					return nil
				}
				sql := strings.NewReplacer("`", "", "\"", "", " ", "").Replace(c.SQL)
				role := c.Ctx.Value(runnerClaimRoleKey{})
				if role == "stop" && released && !readSignalled && strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, "FROMaction_taskWHERE") {
					readSignalled = true
					staleRead <- struct{}{}
					<-resumeRead
				}
				if role == "stop" && !attemptSignalled && strings.HasPrefix(sql, "UPDATEaction_run_attempt") {
					attemptSignalled = true
					attemptHeld <- struct{}{}
					<-resumeStop
				}
				if role == "fetch" && isRunnerClaimLock(c.SQL) {
					runnerHeld <- struct{}{}
				}
				return nil
			}})
			t.Cleanup(func() {
				select {
				case resumeRead <- struct{}{}:
				default:
				}
				select {
				case resumeStop <- struct{}{}:
				default:
				}
			})
			stopDone := make(chan error, 1)
			stopCtx := context.WithValue(ctx, runnerClaimRoleKey{}, "stop")
			go func() {
				stopDone <- db.WithTx(stopCtx, func(ctx context.Context) error {
					db.GetEngine(ctx).Context(ctx)
					return StopTask(ctx, task.ID, StatusCancelled)
				})
			}()
			if released {
				awaitRunnerClaim(t, staleRead)
				require.NoError(t, ReleaseTaskForRunner(t.Context(), task))
				unittest.AssertNotExistsBean(t, &ActionTask{ID: task.ID})
				resumeRead <- struct{}{}
			}
			awaitRunnerClaim(t, attemptHeld)
			fetchDone := make(chan runnerClaimResult, 1)
			fetchCtx := context.WithValue(ctx, runnerClaimRoleKey{}, "fetch")
			go func() {
				var result runnerClaimResult
				if released {
					result.task, result.ok, result.err = claimJobForRunner(fetchCtx, runner, &secondJob)
				} else {
					result.task, result.ok, result.err = CreateTaskForRunner(fetchCtx, runner)
				}
				fetchDone <- result
			}()
			awaitRunnerClaim(t, runnerHeld)
			resumeStop <- struct{}{}
			stopErr := awaitRunnerClaim(t, stopDone)
			fetchResult := awaitRunnerClaim(t, fetchDone)
			for _, err := range []error{stopErr, fetchResult.err} {
				if pgError, ok := errors.AsType[*pq.Error](err); ok {
					t.Logf("真实PG交错错误 SQLSTATE=%s message=%s detail=%s", pgError.Code, pgError.Message, pgError.Detail)
				}
			}
			require.NoError(t, stopErr)
			require.NoError(t, fetchResult.err)
			if released {
				require.True(t, fetchResult.ok)
				unittest.AssertExistsAndLoadBean(t, &ActionRunner{ID: runner.ID})
				unittest.AssertExistsAndLoadBean(t, &ActionTask{ID: fetchResult.task.ID, RunnerID: runner.ID})
			} else {
				assert.False(t, fetchResult.ok)
				assert.Nil(t, fetchResult.task)
				unittest.AssertNotExistsBean(t, &ActionRunner{ID: runner.ID})
			}
		})
	}
}
