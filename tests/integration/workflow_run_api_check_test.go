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
}

func testAPIWorkflowRunBasic(t *testing.T, apiRootURL, userUsername string, runID int64, scope ...auth_model.AccessTokenScope) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, userUsername, scope...)

	apiRunsURL := fmt.Sprintf("%s/%s", apiRootURL, "runs")
	req := NewRequest(t, "GET", apiRunsURL).AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionWorkflowRunsResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	foundRun := false

	for _, run := range runnerList.Entries {
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
		apiRun := api.ActionWorkflowRun{}
		DecodeJSON(t, runResp, &apiRun)
		assert.Equal(t, run.ID, apiRun.ID)
		assert.Equal(t, run.Status, apiRun.Status)
		assert.Equal(t, run.Conclusion, apiRun.Conclusion)
		assert.Equal(t, run.Event, apiRun.Event)

		// Verify jobs list works
		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := api.ActionWorkflowJobsResponse{}
		DecodeJSON(t, jobsResp, &jobList)

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
