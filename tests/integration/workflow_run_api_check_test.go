// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIWorkflowRun(t *testing.T) {
	t.Run("AdminRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/admin/actions", "User1", 802, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("UserRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/user/actions", "User2", 803, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("OrgRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/orgs/org3/actions", "User1", 802, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("RepoRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/repos/org3/repo5/actions", "User2", 802, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("RepoWorkflowRuns", func(t *testing.T) {
		testAPIWorkflowRunsByWorkflowID(t, "org3", "repo5", "test.yaml", "User2", 802, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("PullRequestsField", testAPIWorkflowRunsPullRequestsField)
}

// testAPIWorkflowRunsPullRequestsField exercises the `pull_requests` field and the
// `exclude_pull_requests` toggle by associating an inserted run with fixture PR
// user2/repo1#3 (head: branch2, base: master).
func testAPIWorkflowRunsPullRequestsField(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	run := &actions_model.ActionRun{
		RepoID:        1,
		OwnerID:       2,
		TriggerUserID: 2,
		WorkflowID:    "pr-assoc.yaml",
		Index:         99001,
		Ref:           "refs/pull/3/head",
		CommitSHA:     "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Event:         webhook_module.HookEventPullRequest,
		TriggerEvent:  "pull_request_target",
		Status:        actions_model.StatusSuccess,
	}
	require.NoError(t, db.Insert(ctx, run))

	token := getUserToken(t, "User2", auth_model.AccessTokenScopeReadRepository)
	runsURL := "/api/v1/repos/user2/repo1/actions/workflows/pr-assoc.yaml/runs"

	req := NewRequest(t, "GET", runsURL).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	list := DecodeJSON(t, resp, api.ActionWorkflowRunsResponse{})

	var got *api.ActionWorkflowRun
	for _, r := range list.Entries {
		if r.ID == run.ID {
			got = r
			break
		}
	}
	require.NotNil(t, got, "inserted PR-triggered run not returned")
	require.Len(t, got.PullRequests, 1)
	pr := got.PullRequests[0]
	assert.Equal(t, int64(3), pr.Number)
	assert.Equal(t, "branch2", pr.Head.Ref)
	assert.Equal(t, "master", pr.Base.Ref)
	assert.Equal(t, int64(1), pr.Base.Repo.ID)
	assert.Equal(t, "repo1", pr.Base.Repo.Name)

	req = NewRequest(t, "GET", runsURL+"?exclude_pull_requests=true").AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	excluded := DecodeJSON(t, resp, api.ActionWorkflowRunsResponse{})
	for _, r := range excluded.Entries {
		if r.ID == run.ID {
			assert.Empty(t, r.PullRequests)
		}
	}
}

func testAPIWorkflowRunsByWorkflowID(t *testing.T, owner, repo, workflowID, userUsername string, expectedRunID int64, scope ...auth_model.AccessTokenScope) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, userUsername, scope...)

	workflowRunsURL := fmt.Sprintf("/api/v1/repos/%s/%s/actions/workflows/%s/runs", owner, repo, workflowID)

	req := NewRequest(t, "GET", workflowRunsURL).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	runList := DecodeJSON(t, resp, api.ActionWorkflowRunsResponse{})

	found := false
	for _, run := range runList.Entries {
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, workflowRunsURL, token, run.ID, "", run.Status, "", "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, workflowRunsURL, token, run.ID, "", "", "", run.HeadBranch, "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, workflowRunsURL, token, run.ID, "", "", run.Event, "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, workflowRunsURL, token, run.ID, "", "", "", "", run.TriggerActor.UserName, "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, workflowRunsURL, token, run.ID, "", "", "", "", run.TriggerActor.UserName, run.HeadSha)
		if run.ID == expectedRunID {
			found = true
		}
	}
	assert.True(t, found, "expected to find run with ID %d in workflow %s runs", expectedRunID, workflowID)

	req = NewRequest(t, "GET", workflowRunsURL+"?exclude_pull_requests=true").AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	excludedList := DecodeJSON(t, resp, api.ActionWorkflowRunsResponse{})
	excludedFound := false
	for _, run := range excludedList.Entries {
		assert.Empty(t, run.PullRequests, "expected pull_requests to be empty when excluded")
		if run.ID == expectedRunID {
			excludedFound = true
		}
	}
	assert.True(t, excludedFound, "expected to find run with ID %d when excluding pull requests", expectedRunID)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/workflows/nonexistent.yaml/runs", owner, repo)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func testAPIWorkflowRunBasic(t *testing.T, apiRootURL, userUsername string, runID int64, scope ...auth_model.AccessTokenScope) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, userUsername, scope...)

	apiRunsURL := fmt.Sprintf("%s/%s", apiRootURL, "runs")
	req := NewRequest(t, "GET", apiRunsURL).AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := DecodeJSON(t, runnerListResp, &api.ActionWorkflowRunsResponse{})

	foundRun := false

	for _, run := range runnerList.Entries {
		if run.ID == 802 {
			// Fixture stores registration event (push) and schedule as trigger; API must expose the trigger as Event.
			assert.Equal(t, "schedule", run.Event)
		}
		// Verify filtering works
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, "", run.Status, "", "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, run.Conclusion, "", "", "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, "", "", "", run.HeadBranch, "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, "", "", run.Event, "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, "", "", "", "", run.TriggerActor.UserName, "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, apiRunsURL, token, run.ID, "", "", "", "", run.TriggerActor.UserName, run.HeadSha)

		// Verify run url works
		req := NewRequest(t, "GET", run.URL).AddTokenAuth(token)
		runResp := MakeRequest(t, req, http.StatusOK)
		apiRun := DecodeJSON(t, runResp, &api.ActionWorkflowRun{})
		assert.Equal(t, run.ID, apiRun.ID)
		assert.Equal(t, run.Status, apiRun.Status)
		assert.Equal(t, run.Conclusion, apiRun.Conclusion)
		assert.Equal(t, run.Event, apiRun.Event)

		// Verify jobs list works
		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := DecodeJSON(t, jobsResp, &api.ActionWorkflowJobsResponse{})

		if run.ID == runID {
			foundRun = true
			assert.Len(t, jobList.Entries, 1)
			for _, job := range jobList.Entries {
				// Check the jobs list of the run
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", run.URL, "jobs"), token, job.ID, "", job.Status)
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", run.URL, "jobs"), token, job.ID, job.Conclusion, "")
				// Check the run independent job list
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", apiRootURL, "jobs"), token, job.ID, "", job.Status)
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", apiRootURL, "jobs"), token, job.ID, job.Conclusion, "")

				// Verify job url works
				req := NewRequest(t, "GET", job.URL).AddTokenAuth(token)
				jobsResp := MakeRequest(t, req, http.StatusOK)
				apiJob := DecodeJSON(t, jobsResp, &api.ActionWorkflowJob{})
				assert.Equal(t, job.ID, apiJob.ID)
				assert.Equal(t, job.RunID, apiJob.RunID)
				assert.Equal(t, job.Status, apiJob.Status)
				assert.Equal(t, job.Conclusion, apiJob.Conclusion)
			}
		}
	}
	assert.True(t, foundRun, "Expected to find run with ID %d", runID)
}

func verifyWorkflowRunCanbeFoundWithStatusFilter(t *testing.T, runAPIURL, token string, id int64, conclusion, status, event, branch, actor, headSHA string) {
	filter := url.Values{}
	if conclusion != "" {
		filter.Add("status", conclusion)
	}
	if status != "" {
		filter.Add("status", status)
	}
	if event != "" {
		filter.Set("event", event)
	}
	if branch != "" {
		filter.Set("branch", branch)
	}
	if actor != "" {
		filter.Set("actor", actor)
	}
	if headSHA != "" {
		filter.Set("head_sha", headSHA)
	}
	req := NewRequest(t, "GET", runAPIURL+"?"+filter.Encode()).AddTokenAuth(token)
	runResp := MakeRequest(t, req, http.StatusOK)
	runList := DecodeJSON(t, runResp, &api.ActionWorkflowRunsResponse{})

	found := false
	for _, run := range runList.Entries {
		if conclusion != "" {
			assert.Equal(t, conclusion, run.Conclusion)
		}
		if status != "" {
			assert.Equal(t, status, run.Status)
		}
		if event != "" {
			assert.Equal(t, event, run.Event)
		}
		if branch != "" {
			assert.Equal(t, branch, run.HeadBranch)
		}
		if actor != "" {
			assert.Equal(t, actor, run.Actor.UserName)
		}
		found = found || run.ID == id
	}
	assert.True(t, found, "Expected to find run with ID %d", id)
}

func verifyWorkflowJobCanbeFoundWithStatusFilter(t *testing.T, runAPIURL, token string, id int64, conclusion, status string) {
	filter := conclusion
	if filter == "" {
		filter = status
	}
	if filter == "" {
		return
	}
	req := NewRequest(t, "GET", runAPIURL+"?status="+filter).AddTokenAuth(token)
	jobListResp := MakeRequest(t, req, http.StatusOK)
	jobList := DecodeJSON(t, jobListResp, &api.ActionWorkflowJobsResponse{})

	found := false
	for _, job := range jobList.Entries {
		if conclusion != "" {
			assert.Equal(t, conclusion, job.Conclusion)
		} else {
			assert.Equal(t, status, job.Status)
		}
		found = found || job.ID == id
	}
	assert.True(t, found, "Expected to find job with ID %d", id)
}
