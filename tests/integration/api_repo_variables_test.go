// Copyright 2024 The Gitea Authors. All rights reserved.
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
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
)

func TestAPIRepoVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateRepoVariable", func(t *testing.T) {
		cases := []struct {
			Name           string
			ExpectedStatus int
		}{
			{
				Name:           "-",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "_",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "TEST_VAR",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "test_var",
				ExpectedStatus: http.StatusConflict,
			},
			{
				Name:           "ci",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "123var",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "var@test",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "github_var",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "gitea_var",
				ExpectedStatus: http.StatusBadRequest,
			},
		}

		for _, c := range cases {
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), c.Name), api.CreateVariableOption{
				Value: "value",
			}).AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)
		}
	})

	t.Run("UpdateRepoVariable", func(t *testing.T) {
		variableName := "test_update_var"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variableName)
		req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{
			Value: "initial_val",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		cases := []struct {
			Name           string
			UpdateName     string
			ExpectedStatus int
		}{
			{
				Name:           "not_found_var",
				ExpectedStatus: http.StatusNotFound,
			},
			{
				Name:           variableName,
				UpdateName:     "1invalid",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "invalid@name",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "ci",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "updated_var_name",
				ExpectedStatus: http.StatusNoContent,
			},
			{
				Name:           variableName,
				ExpectedStatus: http.StatusNotFound,
			},
			{
				Name:           "updated_var_name",
				ExpectedStatus: http.StatusNoContent,
			},
		}

		for _, c := range cases {
			req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), c.Name), api.UpdateVariableOption{
				Name:  c.UpdateName,
				Value: "updated_val",
			}).AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)
		}
	})

	t.Run("DeleteRepoVariable", func(t *testing.T) {
		variableName := "test_delete_var"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variableName)

		req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{
			Value: "initial_val",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
