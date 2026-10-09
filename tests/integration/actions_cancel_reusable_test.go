// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	runnerv1 "gitea.dev/actionslib/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cancelReusableCaller = `name: Caller
on: push
concurrency:
  group: ci-${{ gitea.ref }}
  cancel-in-progress: true
jobs:
  ci:
    uses: ./.gitea/workflows/reusable.yaml
`

const cancelReusableCallee = `name: Reusable
on:
  workflow_call:
jobs:
  child:
    runs-on: ubuntu-latest
    steps:
      - run: echo child
`

// cancelledReusableSetup leaves run1 (a reusable caller with a running child task) cancelled by a
// newer push, so that run2 waits for the same concurrency group. It returns the runner, the
// child's task and both runs.
func cancelledReusableSetup(t *testing.T, user *user_model.User, token, repoName string, staleRunner bool) (*repo_model.Repository, *mockRunner, *runnerv1.Task, *actions_model.ActionRun, *actions_model.ActionRun) {
	apiRepo := createActionsTestRepo(t, token, repoName, false)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

	runner := newMockRunner()
	runner.capabilities = []string{"cancelling"} // like gitea-runner: it acknowledges a cancel
	runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

	createRepoWorkflowFile(t, user, token, repo, ".gitea/workflows/reusable.yaml", cancelReusableCallee)
	createRepoWorkflowFile(t, user, token, repo, ".gitea/workflows/caller.yaml", cancelReusableCaller)

	// the push of caller.yaml starts run1, its child job gets a running task
	task1 := runner.fetchTask(t)
	_, _, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
	require.Equal(t, actions_model.StatusRunning, run1.Status)

	if staleRunner {
		// the runner has not reported for longer than taskReportTimeout (it stalled), so Gitea
		// will not wait for an acknowledgement when it cancels the task
		_, err := db.GetEngine(t.Context()).ID(task1.Id).Cols("updated").NoAutoTime().Update(&actions_model.ActionTask{Updated: timeutil.TimeStampNow().AddDuration(-2 * time.Minute)})
		require.NoError(t, err)
	}

	// a second push to the same ref cancels run1 and creates run2, which waits for the group
	createRepoWorkflowFile(t, user, token, repo, "second.txt", "second push")
	var run2 *actions_model.ActionRun
	require.Eventually(t, func() bool {
		run2 = unittest.GetBean(t, &actions_model.ActionRun{RepoID: repo.ID, Index: run1.Index + 1})
		return run2 != nil
	}, 10*time.Second, 100*time.Millisecond, "the second push did not create a run")

	// Gitea cancels the caller job at once. The child with a running task goes to "cancelling"
	// (the runner must acknowledge), or straight to "cancelled" when the runner went quiet.
	want := actions_model.StatusCancelling
	if staleRunner {
		want = actions_model.StatusCancelled
	}
	require.Eventually(t, func() bool {
		run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		return run1.Status == want
	}, 10*time.Second, 100*time.Millisecond, "run1 should be %s, is %s", want, run1.Status)
	return repo, runner, task1, run1, run2
}

// A run whose reusable child was cancelled mid-run must settle once the runner acknowledges the
// cancel, and the run waiting for its concurrency group must then start.
func TestActionsCancelledReusableRunSettles(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		_, runner, task1, run1, _ := cancelledReusableSetup(t, user2, token, "cancel-reusable-settles", false)

		// the runner acknowledges the cancel
		runner.execTask(t, task1, &mockTaskOutcome{result: runnerv1.Result_RESULT_CANCELLED})

		// T0: every job of run1 is done, so the run is cancelled
		assert.Eventually(t, func() bool {
			run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
			return run1.Status == actions_model.StatusCancelled
		}, 5*time.Second, 100*time.Millisecond, "run1 stays %s after its last job was cancelled", run1.Status)

		// T3: the run waiting for the group starts without any manual step
		task2 := runner.tryFetchTask(t, 5*time.Second)
		assert.NotNil(t, task2, "the run waiting for the concurrency group never started")
	})
}

// A run whose jobs are all done but that is still "cancelling" is closed by force-cancel, and the
// run waiting for its concurrency group must start.
func TestActionsForceCancelWakesConcurrencyWaiter(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repo, runner, task1, run1, run2 := cancelledReusableSetup(t, user2, token, "cancel-reusable-force", false)

		// Put run1 in the state that was seen in production: its child task and job are done,
		// the run is still cancelling. Set directly, so this test does not depend on how the
		// run got there.
		_, job1, _ := getTaskAndJobAndRunByTaskID(t, task1.Id)
		now := timeutil.TimeStampNow()
		_, err := db.GetEngine(t.Context()).ID(task1.Id).Cols("status", "stopped").Update(&actions_model.ActionTask{Status: actions_model.StatusCancelled, Stopped: now})
		require.NoError(t, err)
		_, err = db.GetEngine(t.Context()).ID(job1.ID).Cols("status", "stopped").Update(&actions_model.ActionRunJob{Status: actions_model.StatusCancelled, Stopped: now})
		require.NoError(t, err)
		run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		require.Equal(t, actions_model.StatusCancelling, run1.Status)

		run2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run2.ID})
		require.Equal(t, actions_model.StatusBlocked, run2.Status, "run2 must still wait for the group before force-cancel")

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/force-cancel", repo.OwnerName, repo.Name, run1.ID)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)

		run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		assert.Equal(t, actions_model.StatusCancelled, run1.Status)

		// T1: the run waiting for the group starts
		task2 := runner.tryFetchTask(t, 5*time.Second)
		assert.NotNil(t, task2, "force-cancel settled the run but the run waiting for its concurrency group never started")
	})
}

// The runner stalled: it has been silent for longer than taskReportTimeout when the newer push
// cancels the run, so the child task is cancelled without an acknowledgement. The run must still
// settle and the waiting run must start.
func TestActionsCancelledReusableRunOfStalledRunner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		_, runner, _, _, _ := cancelledReusableSetup(t, user2, token, "cancel-reusable-stalled", true)

		task2 := runner.tryFetchTask(t, 5*time.Second)
		assert.NotNil(t, task2, "the run waiting for the concurrency group never started")
	})
}

// In production the caller job is already "cancelled" (not "cancelling") when the runner
// acknowledges the cancel of the child. The run must still settle.
func TestActionsCancelledReusableRunSettlesWithCancelledCaller(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getTokenForLoggedInUser(t, loginUser(t, user2.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		_, runner, task1, run1, _ := cancelledReusableSetup(t, user2, token, "cancel-reusable-caller-cancelled", false)

		var caller actions_model.ActionRunJob
		has, err := db.GetEngine(t.Context()).Where("run_id = ? AND is_reusable_caller = ?", run1.ID, true).Get(&caller)
		require.NoError(t, err)
		require.True(t, has)
		_, err = db.GetEngine(t.Context()).ID(caller.ID).Cols("status", "stopped").Update(&actions_model.ActionRunJob{Status: actions_model.StatusCancelled, Stopped: timeutil.TimeStampNow()})
		require.NoError(t, err)

		runner.execTask(t, task1, &mockTaskOutcome{result: runnerv1.Result_RESULT_CANCELLED})

		assert.Eventually(t, func() bool {
			run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
			return run1.Status == actions_model.StatusCancelled
		}, 5*time.Second, 100*time.Millisecond, "run1 stays %s after its last job was cancelled", run1.Status)
		assert.NotNil(t, runner.tryFetchTask(t, 5*time.Second), "the run waiting for the concurrency group never started")
	})
}
