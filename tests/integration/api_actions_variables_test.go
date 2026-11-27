// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIActionsVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/admin/actions/variables").AddTokenAuth(getUserToken(t, "user2", auth_model.AccessTokenScopeAll)), http.StatusForbidden)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	for _, scope := range []struct {
		name            string
		url             string
		token           string
		ownerID, repoID int64
	}{
		{"Repo", "/api/v1/repos/user2/repo1/actions/variables", getTokenForLoggedInUser(t, loginRepoAdmin(t, repo), auth_model.AccessTokenScopeWriteRepository), 0, repo.ID},
		{"Org", "/api/v1/orgs/org3/actions/variables", getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization), 3, 0},
		{"User", "/api/v1/user/actions/variables", getUserToken(t, "user1", auth_model.AccessTokenScopeWriteUser), 1, 0},
		{"Admin", "/api/v1/admin/actions/variables", getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin), 0, 0},
	} {
		t.Run(scope.name, func(t *testing.T) {
			createVariable := func(name string, expectedStatus int) {
				MakeRequest(t, NewRequestWithJSON(t, "POST", scope.url+"/"+name, api.CreateVariableOption{Value: "value"}).AddTokenAuth(scope.token), expectedStatus)
			}
			updateVariable := func(name, newName string, expectedStatus int) {
				MakeRequest(t, NewRequestWithJSON(t, "PUT", scope.url+"/"+name, api.UpdateVariableOption{Name: newName, Value: "updated"}).AddTokenAuth(scope.token), expectedStatus)
			}

			createVariable("-", http.StatusBadRequest)
			createVariable("TEST_VAR", http.StatusCreated)
			createVariable("test_var", http.StatusConflict)
			updateVariable("not_found_var", "", http.StatusNotFound)
			updateVariable("TEST_VAR", "1invalid", http.StatusBadRequest)
			updateVariable("TEST_VAR", "renamed_var", http.StatusNoContent)
			updateVariable("renamed_var", "", http.StatusNoContent)

			expected := &api.ActionVariable{OwnerID: scope.ownerID, RepoID: scope.repoID, Name: "RENAMED_VAR", Data: "updated"}
			resp := MakeRequest(t, NewRequest(t, "GET", scope.url+"/renamed_var").AddTokenAuth(scope.token), http.StatusOK)
			assert.Equal(t, expected, DecodeJSON(t, resp, &api.ActionVariable{}))
			resp = MakeRequest(t, NewRequest(t, "GET", scope.url).AddTokenAuth(scope.token), http.StatusOK)
			assert.Equal(t, []*api.ActionVariable{expected}, DecodeJSON(t, resp, []*api.ActionVariable{}))

			MakeRequest(t, NewRequest(t, "DELETE", scope.url+"/renamed_var").AddTokenAuth(scope.token), http.StatusNoContent)
			MakeRequest(t, NewRequest(t, "DELETE", scope.url+"/renamed_var").AddTokenAuth(scope.token), http.StatusNotFound)
		})
	}
}
