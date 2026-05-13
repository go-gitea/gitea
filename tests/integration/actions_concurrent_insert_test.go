// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	actions_service "code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestActionsInsertRunDeadlockWithConcurrentUpdateTask reproduces the deadlock between PrepareRunAndInsert and UpdateTaskByState
// reported in https://github.com/go-gitea/gitea/issues/36234
func TestActionsInsertRunDeadlockWithConcurrentUpdateTask(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := t.Context()

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-deadlock-repro", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		// register a real repo-scoped runner
		const runnerName = "deadlock-runner"
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user.Name, repo.Name, runnerName, []string{"ubuntu-latest"}, false)
		runnerBean := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: runnerName, RepoID: repo.ID})
		testRunnerID := runnerBean.ID

		const (
			wfTreePath   = ".gitea/workflows/deadlock-repro.yml"
			workflowYAML = `name: deadlock-repro
on:
  push:
    paths:
      - 'src/**'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`
		)

		opts := getWorkflowCreateFileOptions(user, repo.DefaultBranch, "create "+wfTreePath, workflowYAML)
		fileResp := createWorkflowFile(t, token, user.Name, repo.Name, wfTreePath, opts)
		require.NotNil(t, fileResp.Commit, "createWorkflowFile commit response")
		headCommitSHA := fileResp.Commit.SHA
		require.NotEmpty(t, headCommitSHA, "createWorkflowFile commit sha")

		const (
			inserters   = 8
			updaters    = 8
			insertsEach = 30
			seedTasks   = updaters * 30
		)

		seededTaskIDs := seedRunningTasksForDeadlockTest(t, ctx, repo, repo.OwnerID, user.ID, testRunnerID, headCommitSHA, seedTasks)

		// build a PushPayload
		eventPayload, err := json.Marshal(&api.PushPayload{
			HeadCommit: &api.PayloadCommit{ID: headCommitSHA},
		})
		require.NoError(t, err, "marshal push payload")
		eventPayloadStr := string(eventPayload)

		var (
			deadlockOnInsert atomic.Int64
			deadlockOnUpdate atomic.Int64
			insertErrors     atomic.Int64
			updateErrors     atomic.Int64
			insertSuccess    atomic.Int64
			updateSuccess    atomic.Int64
		)

		taskQueue := make(chan int64, seedTasks)
		for _, tid := range seededTaskIDs {
			taskQueue <- tid
		}
		close(taskQueue)

		startBarrier := make(chan struct{})
		var wg sync.WaitGroup

		for i := range inserters {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				<-startBarrier
				for j := range insertsEach {
					run := &actions_model.ActionRun{
						Title:         fmt.Sprintf("deadlock-test w=%d j=%d", workerID, j),
						RepoID:        repo.ID,
						OwnerID:       repo.OwnerID,
						WorkflowID:    "deadlock-repro.yml",
						TriggerUserID: user.ID,
						Ref:           "refs/heads/" + repo.DefaultBranch,
						CommitSHA:     headCommitSHA,
						Event:         webhook_module.HookEventPush,
						EventPayload:  eventPayloadStr,
						TriggerEvent:  "push",
						Status:        actions_model.StatusWaiting,
					}
					err := actions_service.PrepareRunAndInsert(ctx, []byte(workflowYAML), run, nil)
					if err == nil {
						insertSuccess.Add(1)
						continue
					}
					insertErrors.Add(1)
					if isDeadlockErr(err) {
						deadlockOnInsert.Add(1)
					}
					t.Logf("inserter w=%d j=%d error: %v", workerID, j, err)
				}
			}(i)
		}

		for i := range updaters {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				<-startBarrier
				for tid := range taskQueue {
					state := &runnerv1.TaskState{
						Id:        tid,
						Result:    runnerv1.Result_RESULT_SUCCESS,
						StoppedAt: timestamppb.Now(),
					}
					if _, err := actions_model.UpdateTaskByState(ctx, testRunnerID, state); err == nil {
						updateSuccess.Add(1)
					} else {
						updateErrors.Add(1)
						if isDeadlockErr(err) {
							deadlockOnUpdate.Add(1)
						}
						t.Logf("updater w=%d task=%d error: %v", workerID, tid, err)
					}
				}
			}(i)
		}

		close(startBarrier)
		wg.Wait()

		totalDeadlocks := deadlockOnInsert.Load() + deadlockOnUpdate.Load()

		t.Logf("inserts: success=%d errors=%d deadlocks=%d",
			insertSuccess.Load(), insertErrors.Load(), deadlockOnInsert.Load())
		t.Logf("updates: success=%d errors=%d deadlocks=%d",
			updateSuccess.Load(), updateErrors.Load(), deadlockOnUpdate.Load())

		assert.Zero(t, totalDeadlocks,
			"deadlock detected: insert=%d update=%d (handleWorkflows would silently drop these workflow runs)",
			deadlockOnInsert.Load(), deadlockOnUpdate.Load())

		assert.Equal(t, int64(inserters*insertsEach), insertSuccess.Load(),
			"lost workflow runs: %d insertions failed", insertErrors.Load())
	})
}

// seedRunningTasksForDeadlockTest inserts `n` (run, attempt, job, task) tuples
func seedRunningTasksForDeadlockTest(t *testing.T, ctx context.Context, repo *repo_model.Repository, ownerID, userID, runnerID int64, seedCommitSHA string, n int) []int64 {
	t.Helper()

	// build a PushPayload
	eventPayloadBytes, err := json.Marshal(&api.PushPayload{
		HeadCommit: &api.PayloadCommit{ID: seedCommitSHA},
	})
	require.NoError(t, err, "marshal seed push payload")
	eventPayloadStr := string(eventPayloadBytes)

	taskIDs := make([]int64, 0, n)
	now := timeutil.TimeStampNow()

	for i := range n {
		// action_run has UNIQUE(repo_id, index); use GetNextResourceIndex so the seed shares the same counter
		idx, err := db.GetNextResourceIndex(ctx, "action_run_index", repo.ID)
		require.NoError(t, err, "seed run index %d", i)

		run := &actions_model.ActionRun{
			Title:         fmt.Sprintf("seed-run-%d", i),
			RepoID:        repo.ID,
			OwnerID:       ownerID,
			WorkflowID:    "seed.yml",
			Index:         idx,
			TriggerUserID: userID,
			Ref:           "refs/heads/" + repo.DefaultBranch,
			CommitSHA:     seedCommitSHA,
			Event:         webhook_module.HookEventPush,
			EventPayload:  eventPayloadStr,
			TriggerEvent:  "push",
			Status:        actions_model.StatusRunning,
			Started:       now,
		}
		require.NoError(t, db.Insert(ctx, run), "seed run %d", i)

		attempt := &actions_model.ActionRunAttempt{
			RepoID:        repo.ID,
			RunID:         run.ID,
			Attempt:       1,
			TriggerUserID: userID,
			Status:        actions_model.StatusRunning,
			Started:       now,
		}
		require.NoError(t, db.Insert(ctx, attempt), "seed attempt %d", i)

		run.LatestAttemptID = attempt.ID
		require.NoError(t, actions_model.UpdateRun(ctx, run, "latest_attempt_id"), "seed run latest_attempt_id %d", i)

		job := &actions_model.ActionRunJob{
			RunID:     run.ID,
			RepoID:    repo.ID,
			OwnerID:   ownerID,
			CommitSHA: seedCommitSHA,
			Name:      "job1",
			JobID:     "job1",
			WorkflowPayload: []byte(`name: seed
on: push
jobs:
    job1:
      runs-on: ubuntu-latest
      steps:
        - run: echo hi
`),
			Status:       actions_model.StatusRunning,
			Started:      now,
			Attempt:      1,
			RunAttemptID: attempt.ID,
			AttemptJobID: 1,
		}
		require.NoError(t, db.Insert(ctx, job), "seed job %d", i)

		task := &actions_model.ActionTask{
			JobID:     job.ID,
			Attempt:   1,
			RunnerID:  runnerID,
			Status:    actions_model.StatusRunning,
			Started:   now,
			RepoID:    repo.ID,
			OwnerID:   ownerID,
			CommitSHA: seedCommitSHA,
		}
		task.GenerateAndFillToken()
		require.NoError(t, db.Insert(ctx, task), "seed task %d", i)

		// backfill TaskID on the job
		job.TaskID = task.ID
		_, err = actions_model.UpdateRunJob(ctx, job, nil, "task_id")
		require.NoError(t, err, "seed job.task_id %d", i)

		taskIDs = append(taskIDs, task.ID)
	}

	return taskIDs
}

// isDeadlockErr matches the deadlock error surface across every backend the test runs against
func isDeadlockErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Deadlock found") ||
		// MySQL / MariaDB: vendor code 1213 ("Deadlock found"), SQLSTATE 40001
		strings.Contains(s, "1213") ||
		strings.Contains(s, "40001") ||
		// PostgreSQL: SQLSTATE 40P01
		strings.Contains(s, "40P01") ||
		// MSSQL: vendor code 1205 ("deadlocked on lock resources")
		strings.Contains(s, "1205") ||
		strings.Contains(s, "deadlocked on lock resources")
}
