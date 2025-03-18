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
	userUsername := "user5"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", "/api/v1/repos/user5/repo4/actions/runs").AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionWorkflowRunsResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	assert.Len(t, runnerList.Entries, 4)

	for _, run := range runnerList.Entries {
		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", run.URL, "jobs")).AddTokenAuth(token)
		jobsResp := MakeRequest(t, req, http.StatusOK)
		jobList := api.ActionWorkflowJobsResponse{}
		DecodeJSON(t, jobsResp, &jobList)

		// assert.NotEmpty(t, jobList.Entries)
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
		// assert.NotEmpty(t, run.ID)
		// assert.NotEmpty(t, run.Status)
		// assert.NotEmpty(t, run.Event)
		// assert.NotEmpty(t, run.WorkflowID)
		// assert.NotEmpty(t, run.HeadBranch)
		// assert.NotEmpty(t, run.HeadSHA)
		// assert.NotEmpty(t, run.CreatedAt)
		// assert.NotEmpty(t, run.UpdatedAt)
		// assert.NotEmpty(t, run.URL)
		// assert.NotEmpty(t, run.HTMLURL)
		// assert.NotEmpty(t, run.PullRequests)
		// assert.NotEmpty(t, run.WorkflowURL)
		// assert.NotEmpty(t, run.HeadCommit)
		// assert.NotEmpty(t, run.HeadRepository)
		// assert.NotEmpty(t, run.Repository)
		// assert.NotEmpty(t, run.HeadRepository)
		// assert.NotEmpty(t, run.HeadRepository.Owner)
		// assert.NotEmpty(t, run.HeadRepository.Name)
		// assert.NotEmpty(t, run.Repository.Owner)
		// assert.NotEmpty(t, run.Repository.Name)
		// assert.NotEmpty(t, run.HeadRepository.Owner.Login)
		// assert.NotEmpty(t, run.HeadRepository.Name)
		// assert.NotEmpty(t, run.Repository.Owner.Login)
		// assert.NotEmpty(t, run.Repository.Name)
	}
}
