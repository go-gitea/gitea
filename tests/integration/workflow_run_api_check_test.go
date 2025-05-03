// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIWorkflowRun(t *testing.T) {
	t.Run("AdminRunner", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/admin/actions/runs", 6, "User1", 802, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("UserRunner", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/user/actions/runs", 1, "User2", 803, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("OrgRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/orgs/org3/actions/runs", 1, "User1", 802, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository)
	})
	t.Run("RepoRuns", func(t *testing.T) {
		testAPIWorkflowRunBasic(t, "/api/v1/repos/org3/repo5/actions/runs", 1, "User2", 802, auth_model.AccessTokenScopeReadRepository)
	})
}

func testAPIWorkflowRunBasic(t *testing.T, runAPIURL string, itemCount int, userUsername string, runID int64, scope ...auth_model.AccessTokenScope) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, userUsername, scope...)

	req := NewRequest(t, "GET", runAPIURL).AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionWorkflowRunsResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	assert.Len(t, runnerList.Entries, itemCount)

	foundRun := false

	for _, run := range runnerList.Entries {
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, runAPIURL, token, run.ID, "", run.Status, "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, runAPIURL, token, run.ID, run.Conclusion, "", "", "")
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, runAPIURL, token, run.ID, "", "", "", run.HeadBranch)
		verifyWorkflowRunCanbeFoundWithStatusFilter(t, runAPIURL, token, run.ID, "", "", run.Event, "")

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := api.ActionWorkflowJobsResponse{}
		DecodeJSON(t, jobsResp, &jobList)

		if run.ID == runID {
			foundRun = true
			assert.Len(t, jobList.Entries, 1)
			for _, job := range jobList.Entries {
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", run.URL, "jobs"), token, job.ID, "", job.Status)
				verifyWorkflowJobCanbeFoundWithStatusFilter(t, fmt.Sprintf("%s/%s", run.URL, "jobs"), token, job.ID, job.Conclusion, "")

				req := NewRequest(t, "GET", job.URL).AddTokenAuth(token)
				jobsResp := MakeRequest(t, req, http.StatusOK)
				apiJob := api.ActionWorkflowJob{}
				DecodeJSON(t, jobsResp, &apiJob)
				assert.Equal(t, job.ID, apiJob.ID)
				assert.Equal(t, job.RunID, apiJob.RunID)
				assert.Equal(t, job.Status, apiJob.Status)
				assert.Equal(t, job.Conclusion, apiJob.Conclusion)
			}
		}
	}
	assert.True(t, foundRun, "Expected to find run with ID %d", runID)
}

func verifyWorkflowRunCanbeFoundWithStatusFilter(t *testing.T, runAPIURL, token string, id int64, conclusion, status, event, branch string) {
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
	req := NewRequest(t, "GET", runAPIURL+"?"+filter.Encode()).AddTokenAuth(token)
	runResp := MakeRequest(t, req, http.StatusOK)
	runList := api.ActionWorkflowRunsResponse{}
	DecodeJSON(t, runResp, &runList)

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
	jobList := api.ActionWorkflowJobsResponse{}
	DecodeJSON(t, jobListResp, &jobList)

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
