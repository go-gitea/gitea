// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
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
)

func TestAPIActionsGetWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802802", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/803", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
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

func TestAPIListWorkflowRunJobsReturnsSteps(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

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
	assert.NoError(t, err)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var jobList api.ActionWorkflowJobsResponse
	err = json.Unmarshal(resp.Body.Bytes(), &jobList)
	assert.NoError(t, err)

	var job198 *api.ActionWorkflowJob
	for _, j := range jobList.Entries {
		if j.ID == 198 {
			job198 = j
			break
		}
	}
	assert.NotNil(t, job198, "expected to find job 198 in run 795 jobs list")

	// Steps must be non-nil and populated when the job has a task with steps
	assert.NotNil(t, job198.Steps, "job steps must not be null")
	assert.NotEmpty(t, job198.Steps, "job must return at least one step when task has steps")
	if len(job198.Steps) > 0 {
		assert.Equal(t, "main", job198.Steps[0].Name, "first step name")
	}
}
