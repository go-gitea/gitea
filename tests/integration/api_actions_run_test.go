// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
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
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test rerun existing run
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test rerun non-existent run
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/rerun", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsRerunWorkflowRunPermissions(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // User without write access
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test rerun without permissions should fail
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIActionsCancelWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test cancel running workflow
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/cancel", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test cancel non-existent run
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/cancel", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsCancelWorkflowRunPermissions(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // User without write access
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test cancel without permissions should fail
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/cancel", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIActionsApproveWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test approve workflow run
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/approve", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test approve non-existent run
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/approve", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsRerunWorkflowJob(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test rerun specific job
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/192/rerun", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test rerun non-existent job
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/rerun", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsGetWorkflowRunLogs(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test get workflow run logs (archive download)
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test get non-existent run logs
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/logs", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsGetWorkflowJobLogs(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/192/logs", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/logs", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIActionsGetWorkflowRunLogsStream(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// Test streaming logs with empty cursor request
	req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName()), strings.NewReader(`{"logCursors": []}`)).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	// Parse response to verify structure
	var logResp map[string]any
	err := json.Unmarshal(resp.Body.Bytes(), &logResp)
	assert.NoError(t, err)
	assert.Contains(t, logResp, "stepsLog")

	// Test streaming logs with cursor request
	req = NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName()), strings.NewReader(`{"logCursors": [{"step": 0, "cursor": 0, "expanded": true}]}`)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// Test streaming logs for non-existent run
	req = NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/logs", repo.FullName()), strings.NewReader(`{"logCursors": []}`)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}
