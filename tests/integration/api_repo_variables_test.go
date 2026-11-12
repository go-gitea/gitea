// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"
)

func TestAPIRepoVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := getTokenForLoggedInUser(t, loginRepoAdmin(t, repo), auth_model.AccessTokenScopeWriteRepository)
	variablesURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables/", repo.FullName())
	createVariable := func(name string, expectedStatus int) {
		option := api.CreateVariableOption{Value: "value"}
		MakeRequest(t, NewRequestWithJSON(t, "POST", variablesURL+name, option).AddTokenAuth(token), expectedStatus)
	}
	updateVariable := func(name, newName string, expectedStatus int) {
		option := api.UpdateVariableOption{Name: newName, Value: "updated"}
		MakeRequest(t, NewRequestWithJSON(t, "PUT", variablesURL+name, option).AddTokenAuth(token), expectedStatus)
	}

	createVariable("-", http.StatusBadRequest)
	createVariable("TEST_VAR", http.StatusCreated)
	createVariable("test_var", http.StatusConflict)
	updateVariable("not_found_var", "", http.StatusNotFound)
	updateVariable("TEST_VAR", "1invalid", http.StatusBadRequest)
	updateVariable("TEST_VAR", "renamed_var", http.StatusNoContent)
	updateVariable("TEST_VAR", "", http.StatusNotFound)
	updateVariable("renamed_var", "", http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "DELETE", variablesURL+"renamed_var").AddTokenAuth(token), http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "DELETE", variablesURL+"renamed_var").AddTokenAuth(token), http.StatusNotFound)
}
