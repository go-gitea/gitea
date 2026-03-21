// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	actions_web "code.gitea.io/gitea/routers/web/repo/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestActionsRoute(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		repo1 := createActionsTestRepo(t, user2Token, "actions-route-test-1", false)
		runner1 := newMockRunner()
		runner1.registerAsRepoRunner(t, user2.Name, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)
		repo2 := createActionsTestRepo(t, user2Token, "actions-route-test-2", false)
		runner2 := newMockRunner()
		runner2.registerAsRepoRunner(t, user2.Name, repo2.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		workflowTreePath := ".gitea/workflows/test.yml"
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

		opts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+workflowTreePath, workflowContent)
		createWorkflowFile(t, user2Token, user2.Name, repo1.Name, workflowTreePath, opts)
		createWorkflowFile(t, user2Token, user2.Name, repo2.Name, workflowTreePath, opts)

		task1 := runner1.fetchTask(t)
		_, job1, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
		task2 := runner2.fetchTask(t)
		_, job2, run2 := getTaskAndJobAndRunByTaskID(t, task2.Id)

		// run1 and job1 belong to repo1, success
		req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run1.ID, job1.ID))
		resp := user2Session.MakeRequest(t, req, http.StatusOK)
		var viewResp actions_web.ViewResponse
		DecodeJSON(t, resp, &viewResp)
		assert.Len(t, viewResp.State.Run.Jobs, 1)
		assert.Equal(t, job1.ID, viewResp.State.Run.Jobs[0].ID)

		// run2 and job2 do not belong to repo1, failure
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run2.ID, job2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run1.ID, job2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run2.ID, job1.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/workflow", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/approve", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/cancel", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/delete", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/artifacts/test.txt", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "DELETE", fmt.Sprintf("/%s/%s/actions/runs/%d/artifacts/test.txt", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// make the tasks complete, then test rerun
		runner1.execTask(t, task1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		runner2.execTask(t, task2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo1.Name, run2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run2.ID, job2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run1.ID, job2.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run2.ID, job1.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
	})
}
