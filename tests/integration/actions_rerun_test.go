// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
)

func TestActionsRerun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-rerun", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		wfTreePath := ".gitea/workflows/actions-rerun-workflow-1.yml"
		wfFileContent := `name: actions-rerun-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/actions-rerun-workflow-1.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job1'
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo 'job2'
`

		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create"+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts)

		// fetch and exec job1
		job1Task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, job1Task.Id)
		runner.execTask(t, job1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// RERUN-FAILURE: the run is not done
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.Index), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusBadRequest)
		// fetch and exec job2
		job2Task := runner.fetchTask(t)
		runner.execTask(t, job2Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// RERUN-1: rerun the run
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.Index), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusOK)
		// fetch and exec job1
		job1TaskR1 := runner.fetchTask(t)
		runner.execTask(t, job1TaskR1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch and exec job2
		job2TaskR1 := runner.fetchTask(t)
		runner.execTask(t, job2TaskR1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// RERUN-2: rerun job1
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo.Name, run.Index, 0), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusOK)
		// job2 needs job1, so rerunning job1 will also rerun job2
		// fetch and exec job1
		job1TaskR2 := runner.fetchTask(t)
		runner.execTask(t, job1TaskR2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch and exec job2
		job2TaskR2 := runner.fetchTask(t)
		runner.execTask(t, job2TaskR2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// RERUN-3: rerun job2
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo.Name, run.Index, 1), map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
		})
		session.MakeRequest(t, req, http.StatusOK)
		// only job2 will rerun
		// fetch and exec job2
		job2TaskR3 := runner.fetchTask(t)
		runner.execTask(t, job2TaskR3, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		runner.fetchNoTask(t)
	})
}
