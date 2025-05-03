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

func TestAPIWorkflowRunRepoApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", "/api/v1/repos/org3/repo5/actions/runs").AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionWorkflowRunsResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	assert.Len(t, runnerList.Entries, 1)

	foundRun := false

	for _, run := range runnerList.Entries {
		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := api.ActionWorkflowJobsResponse{}
		DecodeJSON(t, jobsResp, &jobList)

		if run.ID == 802 {
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
