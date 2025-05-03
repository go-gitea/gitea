// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
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
		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := api.ActionWorkflowJobsResponse{}
		DecodeJSON(t, jobsResp, &jobList)

		if run.ID == runID {
			foundRun = true
			assert.Len(t, jobList.Entries, 1)
			for _, job := range jobList.Entries {
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
	assert.True(t, foundRun, "Expected to find run with ID 802")
}
