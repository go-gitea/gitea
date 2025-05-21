// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

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
