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
	actions_web "code.gitea.io/gitea/routers/web/repo/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestActionsRoute(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("testActionsRouteForIDBasedURL", testActionsRouteForIDBasedURL)
		t.Run("testActionsRouteForLegacyIndexBasedURL", testActionsRouteForLegacyIndexBasedURL)
	})
}

func testActionsRouteForIDBasedURL(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2Session := loginUser(t, user2.Name)
	user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	repo1 := createActionsTestRepo(t, user2Token, "actions-route-id-url-1", false)
	runner1 := newMockRunner()
	runner1.registerAsRepoRunner(t, user2.Name, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)
	repo2 := createActionsTestRepo(t, user2Token, "actions-route-id-url-2", false)
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

	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo1.Name, run1.ID))
	user2Session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo1.Name, 999999))
	user2Session.MakeRequest(t, req, http.StatusNotFound)

	// run1 and job1 belong to repo1, success
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run1.ID, job1.ID))
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
}

func testActionsRouteForLegacyIndexBasedURL(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2Session := loginUser(t, user2.Name)
	user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	repo := createActionsTestRepo(t, user2Token, "actions-route-legacy-index-url", false)

	generateTestRunAndJobs := func(title string, runID, runIndex int64, jobIDs ...int64) (*actions_model.ActionRun, []*actions_model.ActionRunJob) {
		run := &actions_model.ActionRun{
			ID:            runID,
			Index:         runIndex,
			RepoID:        repo.ID,
			OwnerID:       user2.ID,
			WorkflowID:    title + ".yml",
			TriggerUserID: user2.ID,
			Status:        actions_model.StatusSuccess,
			CommitSHA:     title + "sha",
		}

		jobs := make([]*actions_model.ActionRunJob, 0, len(jobIDs))
		for _, jobID := range jobIDs {
			job := &actions_model.ActionRunJob{
				ID:      jobID,
				RunID:   run.ID,
				RepoID:  repo.ID,
				OwnerID: user2.ID,
				Name:    fmt.Sprintf("%s-job-%d", title, jobID),
				Status:  actions_model.StatusSuccess,
			}
			jobs = append(jobs, job)
		}
		return run, jobs
	}

	smallIDRun, smallIDJobs := generateTestRunAndJobs("small-id-run", 80, 20, 170)             // run_id=80, run_index=20
	otherSmallRun, otherSmallJobs := generateTestRunAndJobs("other-small", 90, 30, 180)        // run_id=90, run_index=30
	normalRun, normalRunJobs := generateTestRunAndJobs("normal", 1500, 900, 1600)              // run_id=1500, run_index=900
	collisionRun, collisionJobs := generateTestRunAndJobs("collision", 2400, 1500, 2600, 2601) // run_id=2400, run_index=1500

	_, err := db.GetEngine(t.Context()).Insert(
		smallIDRun,
		smallIDJobs[0],
		otherSmallRun,
		otherSmallJobs[0],
		normalRun,
		normalRunJobs[0],
		collisionRun,
		collisionJobs[0],
		collisionJobs[1],
	)
	assert.NoError(t, err)

	t.Run("OnlyRunID", func(t *testing.T) {
		// ID-based URLs must be valid
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, smallIDRun.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("OnlyRunIndex", func(t *testing.T) {
		// legacy run index should redirect to the ID-based URL
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.Index))
		resp := user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.ID), resp.Header().Get("Location"))

		// Best-effort compatibility prefers the run ID when the same number also exists as a legacy run index in the repo.
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), fmt.Sprintf(`data-run-id="%d"`, normalRun.ID))
	})

	t.Run("RunIDAndJobID", func(t *testing.T) {
		// ID-based URLs must be valid
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, smallIDRun.ID, smallIDJobs[0].ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, normalRun.ID, normalRunJobs[0].ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("RunIndexAndJobIndex", func(t *testing.T) {
		// legacy job index 0 should redirect to the first job's ID
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/0", user2.Name, repo.Name, collisionRun.Index))
		resp := user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, collisionRun.ID, collisionJobs[0].ID), resp.Header().Get("Location"))

		// legacy job index 1 should redirect to the second job's ID
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/1", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, collisionRun.ID, collisionJobs[1].ID), resp.Header().Get("Location"))
	})

	t.Run("InvalidURLs", func(t *testing.T) {
		// the job ID from a different run should not match
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, smallIDRun.ID, otherSmallJobs[0].ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// resolve the run by index first and then return not found because the job index is out-of-range
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/2", user2.Name, repo.Name, normalRun.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// an out-of-range job index should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/2", user2.Name, repo.Name, collisionRun.Index))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// a missing run number should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, 999999))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// a missing legacy run index should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/0", user2.Name, repo.Name, 999999))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
	})
}
