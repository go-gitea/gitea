// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIActionsGetWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("GetRun", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802802", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/803", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("GetJobSteps", func(t *testing.T) {
		// Insert task steps for task_id 53 (job 198) so the API can return them once the backend loads them
		_, err := db.GetEngine(t.Context()).Insert(&actions_model.ActionTaskStep{
			Name:    "main",
			TaskID:  53,
			Index:   0,
			RepoID:  repo.ID,
			Status:  actions_model.StatusSuccess,
			Started: timeutil.TimeStamp(1683636528),
			Stopped: timeutil.TimeStamp(1683636626),
		})
		require.NoError(t, err)

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs", repo.FullName())).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var jobList api.ActionWorkflowJobsResponse
		err = json.Unmarshal(resp.Body.Bytes(), &jobList)
		require.NoError(t, err)

		job198Idx := slices.IndexFunc(jobList.Entries, func(job *api.ActionWorkflowJob) bool { return job.ID == 198 })
		require.NotEqual(t, -1, job198Idx, "expected to find job 198 in run 795 jobs list")
		job198 := jobList.Entries[job198Idx]
		require.NotEmpty(t, job198.Steps, "job must return at least one step when task has steps")
		assert.Equal(t, "main", job198.Steps[0].Name, "first step name")
	})
}

func TestAPIActionsGetWorkflowJob(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198198", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/196", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsDeleteRunCheckPermission(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNotFound)
}

func TestAPIActionsDeleteRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	testAPIActionsDeleteRunListArtifacts(t, repo, token, 2)
	testAPIActionsDeleteRunListTasks(t, repo, token, true)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNoContent)

	testAPIActionsDeleteRunListArtifacts(t, repo, token, 0)
	testAPIActionsDeleteRunListTasks(t, repo, token, false)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNotFound)
}

func TestAPIActionsDeleteRunRunning(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)
}

func testAPIActionsDeleteRun(t *testing.T, repo *repo_model.Repository, token string, expected int) {
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, expected)
}

func testAPIActionsDeleteRunListArtifacts(t *testing.T, repo *repo_model.Repository, token string, artifacts int) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/artifacts", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	assert.Len(t, listResp.Entries, artifacts)
}

func testAPIActionsDeleteRunListTasks(t *testing.T, repo *repo_model.Repository, token string, expected bool) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/tasks", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionTaskResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	findTask1 := false
	findTask2 := false
	for _, entry := range listResp.Entries {
		if entry.ID == 53 {
			findTask1 = true
			continue
		}
		if entry.ID == 54 {
			findTask2 = true
			continue
		}
	}
	assert.Equal(t, expected, findTask1)
	assert.Equal(t, expected, findTask2)
}

func TestAPIActionsRerunWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		var rerunResp api.ActionWorkflowRun
		err := json.Unmarshal(resp.Body.Bytes(), &rerunResp)
		require.NoError(t, err)
		assert.Equal(t, int64(795), rerunResp.ID)
		assert.Equal(t, "queued", rerunResp.Status)
		assert.Equal(t, "c2d72f548424103f01ee1dc02889c1e2bff816b0", rerunResp.HeadSha)

		run, err := actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		assert.Equal(t, timeutil.TimeStamp(0), run.Started)
		assert.Equal(t, timeutil.TimeStamp(0), run.Stopped)

		job198, err := actions_model.GetRunJobByID(t.Context(), 198)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, job198.Status)
		assert.Equal(t, int64(0), job198.TaskID)

		job199, err := actions_model.GetRunJobByID(t.Context(), 199)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, job199.Status)
		assert.Equal(t, int64(0), job199.TaskID)
	})

	t.Run("ForbiddenWithoutWriteScope", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(readToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIActionsRerunWorkflowRunNotDone(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/rerun", repo.FullName())).
		AddTokenAuth(writeToken)
	MakeRequest(t, req, http.StatusBadRequest)
}

func TestAPIActionsRerunWorkflowJob(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		var rerunResp api.ActionWorkflowJob
		err := json.Unmarshal(resp.Body.Bytes(), &rerunResp)
		require.NoError(t, err)
		assert.Equal(t, int64(199), rerunResp.ID)
		assert.Equal(t, "queued", rerunResp.Status)

		run, err := actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, run.Status)

		job198, err := actions_model.GetRunJobByID(t.Context(), 198)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusSuccess, job198.Status)
		assert.Equal(t, int64(53), job198.TaskID)

		job199, err := actions_model.GetRunJobByID(t.Context(), 199)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, job199.Status)
		assert.Equal(t, int64(0), job199.TaskID)
	})

	t.Run("ForbiddenWithoutWriteScope", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).
			AddTokenAuth(readToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("NotFoundJob", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIActionsRerunWorkflowJobRunNotDone(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/jobs/194/rerun", repo.FullName())).
		AddTokenAuth(writeToken)
	MakeRequest(t, req, http.StatusBadRequest)
}
