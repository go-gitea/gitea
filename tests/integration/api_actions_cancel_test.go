// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestAPICancelActionRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		apiRepo := createActionsTestRepo(t, token, "actions-api-cancel-run", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		wfRunningPath := ".gitea/workflows/api-cancel-running.yml"
		wfQueuedPath := ".gitea/workflows/api-cancel-queued.yml"
		wfContent := `name: api-cancel
on:
  push:
    paths:
      - '%s'
jobs:
  job1:
    runs-on: %s
    steps:
      - run: echo cancel
`

		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfRunningPath, fmt.Sprintf(wfContent, wfRunningPath, "ubuntu-latest"))
		createWorkflowFile(t, token, user2.Name, repo.Name, wfRunningPath, opts)

		runningTask := runner.fetchTask(t)
		_, _, runningRun := getTaskAndJobAndRunByTaskID(t, runningTask.Id)
		assert.Equal(t, actions_model.StatusRunning, runningRun.Status)

		opts = getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfQueuedPath, fmt.Sprintf(wfContent, wfQueuedPath, "missing-runner"))
		createWorkflowFile(t, token, user2.Name, repo.Name, wfQueuedPath, opts)

		queuedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{
			RepoID:     repo.ID,
			WorkflowID: "api-cancel-queued.yml",
		})
		assert.Equal(t, actions_model.StatusWaiting, queuedRun.Status)

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/cancel", user2.Name, repo.Name, queuedRun.ID)).
			AddTokenAuth(readToken)
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/cancel", user2.Name, repo.Name, queuedRun.ID)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusAccepted)
		assertRunJobsStatusEventually(t, repo.ID, queuedRun.ID, actions_model.StatusCancelled)
		assertRunStatusEventually(t, queuedRun.ID, actions_model.StatusCancelled)

		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/cancel", user2.Name, repo.Name, runningRun.ID)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusAccepted)
		assertRunJobsStatusEventually(t, repo.ID, runningRun.ID, actions_model.StatusCancelled)
		assertRunStatusEventually(t, runningRun.ID, actions_model.StatusCancelled)

		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/cancel", user2.Name, repo.Name, runningRun.ID)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)
	})
}

func assertRunJobsStatusEventually(t *testing.T, repoID, runID int64, want actions_model.Status) {
	t.Helper()

	assert.Eventually(t, func() bool {
		jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(t.Context(), repoID, runID)
		if err != nil || len(jobs) == 0 {
			return false
		}
		for _, job := range jobs {
			if job.Status != want {
				return false
			}
		}
		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func assertRunStatusEventually(t *testing.T, runID int64, want actions_model.Status) {
	t.Helper()

	assert.Eventually(t, func() bool {
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
		return run.Status == want
	}, 5*time.Second, 100*time.Millisecond)
}
