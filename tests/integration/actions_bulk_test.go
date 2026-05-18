// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActionsBulkCancel verifies that POST /{owner}/{repo}/actions/runs/cancel
// cancels all waiting/running runs, optionally filtered by workflow or status.
func TestActionsBulkCancel(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-bulk-cancel-test", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowContent := `name: test
on:
  push:
    paths:
      - '.gitea/workflows/test.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
`
		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create workflow", workflowContent)
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/test.yml", opts)

		// Fetch the task so the run transitions to "running".
		task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)

		// Confirm the run is running before we bulk-cancel.
		require.Equal(t, actions_model.StatusRunning, run.Status)

		// Bulk-cancel all running/waiting runs in the repo.
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/cancel", user2.Name, apiRepo.Name), map[string]string{})
		session.MakeRequest(t, req, http.StatusOK)

		// Complete the task from the runner side so the DB reflects the final state.
		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_CANCELLED})

		// Reload from DB and verify the run is now cancelled.
		updatedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusCancelled, updatedRun.Status)
	})
}

// TestActionsBulkCancelByWorkflow verifies that the workflow filter is respected:
// only runs for the specified workflow are cancelled.
func TestActionsBulkCancelByWorkflow(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-bulk-cancel-wf-test", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		makeWorkflow := func(name string) string {
			return fmt.Sprintf(`name: %s
on:
  push:
    paths:
      - '.gitea/workflows/%s.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`, name, name)
		}

		opts1 := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create wf1", makeWorkflow("wf1"))
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/wf1.yml", opts1)
		task1 := runner.fetchTask(t)
		_, _, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)

		opts2 := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create wf2", makeWorkflow("wf2"))
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/wf2.yml", opts2)
		task2 := runner.fetchTask(t)
		_, _, run2 := getTaskAndJobAndRunByTaskID(t, task2.Id)

		// Cancel only wf1 runs.
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/cancel", user2.Name, apiRepo.Name), map[string]string{
			"workflow": "wf1.yml",
		})
		session.MakeRequest(t, req, http.StatusOK)

		runner.execTask(t, task1, &mockTaskOutcome{result: runnerv1.Result_RESULT_CANCELLED})
		runner.execTask(t, task2, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

		updatedRun1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		assert.Equal(t, actions_model.StatusCancelled, updatedRun1.Status)

		// run2 was not targeted by the bulk cancel.
		updatedRun2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run2.ID})
		assert.NotEqual(t, actions_model.StatusCancelled, updatedRun2.Status)
	})
}

// TestActionsBulkDelete verifies that POST /{owner}/{repo}/actions/runs/delete
// removes all completed runs and that trying to fetch them afterward returns 404.
func TestActionsBulkDelete(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-bulk-delete-test", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowContent := `name: test
on:
  push:
    paths:
      - '.gitea/workflows/test.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
`
		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create workflow", workflowContent)
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/test.yml", opts)

		task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)
		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

		// Verify the run exists and is done.
		completedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		require.True(t, completedRun.Status.IsDone())

		// Bulk-delete all completed runs.
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/delete", user2.Name, apiRepo.Name), map[string]string{})
		session.MakeRequest(t, req, http.StatusOK)

		// The run must be gone from the DB.
		runs, err := db.Find[actions_model.ActionRun](t.Context(), actions_model.FindRunOptions{
			RepoID: apiRepo.ID,
		})
		require.NoError(t, err)
		for _, r := range runs {
			assert.NotEqual(t, run.ID, r.ID, "deleted run should not appear in list")
		}

		// The run detail page should return 404.
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, apiRepo.Name, run.ID))
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

// TestActionsBulkDeleteSkipsRunning verifies that a running run is NOT deleted
// by bulk-delete even when no status filter is applied.
func TestActionsBulkDeleteSkipsRunning(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-bulk-delete-running-test", false)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, apiRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowContent := `name: test
on:
  push:
    paths:
      - '.gitea/workflows/test.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
`
		opts := getWorkflowCreateFileOptions(user2, apiRepo.DefaultBranch, "create workflow", workflowContent)
		createWorkflowFile(t, token, user2.Name, apiRepo.Name, ".gitea/workflows/test.yml", opts)

		task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)

		// Run is in running state — do NOT complete it.
		require.Equal(t, actions_model.StatusRunning, run.Status)

		// Bulk-delete should silently skip it.
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/delete", user2.Name, apiRepo.Name), map[string]string{})
		session.MakeRequest(t, req, http.StatusOK)

		// Run must still exist.
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})

		// Clean up: let the task finish.
		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
	})
}
