// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoEnvironments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	baseURL := fmt.Sprintf("/api/v1/repos/%s/environments", repo.FullName())

	t.Run("ListEmpty", func(t *testing.T) {
		req := NewRequest(t, "GET", baseURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var envs []*api.ActionEnvironment
		DecodeJSON(t, resp, &envs)
		assert.Empty(t, envs)
	})

	t.Run("Create", func(t *testing.T) {
		cases := []struct {
			name           string
			payload        api.CreateEnvironmentOption
			expectedStatus int
		}{
			{
				name:           "empty name rejected",
				payload:        api.CreateEnvironmentOption{Name: ""},
				expectedStatus: http.StatusUnprocessableEntity,
			},
			{
				name:           "valid without branch rule",
				payload:        api.CreateEnvironmentOption{Name: "staging"},
				expectedStatus: http.StatusCreated,
			},
			{
				name:           "valid with branch rule",
				payload:        api.CreateEnvironmentOption{Name: "production", ProtectedBranches: "main"},
				expectedStatus: http.StatusCreated,
			},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", baseURL, c.payload).AddTokenAuth(token)
				MakeRequest(t, req, c.expectedStatus)
			})
		}
	})

	// helper: create a fresh env for sub-tests that need one
	createEnv := func(t *testing.T, name, protectedBranches string) *api.ActionEnvironment {
		t.Helper()
		req := NewRequestWithJSON(t, "POST", baseURL, api.CreateEnvironmentOption{
			Name:              name,
			ProtectedBranches: protectedBranches,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		var env api.ActionEnvironment
		DecodeJSON(t, resp, &env)
		return &env
	}

	t.Run("GetAndList", func(t *testing.T) {
		env := createEnv(t, "get-test", "main")

		// GET single
		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", baseURL, env.Name)).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var got api.ActionEnvironment
		DecodeJSON(t, resp, &got)
		assert.Equal(t, env.Name, got.Name)
		assert.Equal(t, "main", got.ProtectedBranches)

		// list includes it
		req = NewRequest(t, "GET", baseURL).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		var envs []*api.ActionEnvironment
		DecodeJSON(t, resp, &envs)
		names := make([]string, len(envs))
		for i, e := range envs {
			names[i] = e.Name
		}
		assert.Contains(t, names, env.Name)
	})

	t.Run("Update", func(t *testing.T) {
		env := createEnv(t, "update-test", "develop")
		url := fmt.Sprintf("%s/%s", baseURL, env.Name)

		req := NewRequestWithJSON(t, "PATCH", url, api.UpdateEnvironmentOption{
			ProtectedBranches: "main,release/*",
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var updated api.ActionEnvironment
		DecodeJSON(t, resp, &updated)
		assert.Equal(t, "main,release/*", updated.ProtectedBranches)
	})

	t.Run("Delete", func(t *testing.T) {
		env := createEnv(t, "delete-test", "")
		url := fmt.Sprintf("%s/%s", baseURL, env.Name)

		req := NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		req := NewRequest(t, "GET", baseURL+"/nonexistent").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIRepoEnvironmentSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// create environment to use in all sub-tests
	envURL := fmt.Sprintf("/api/v1/repos/%s/environments", repo.FullName())
	req := NewRequestWithJSON(t, "POST", envURL, api.CreateEnvironmentOption{Name: "sec-env"}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var env api.ActionEnvironment
	DecodeJSON(t, resp, &env)
	secretsURL := fmt.Sprintf("%s/%s/secrets", envURL, env.Name)

	t.Run("ListEmpty", func(t *testing.T) {
		req := NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var secrets []*api.EnvironmentSecret
		DecodeJSON(t, resp, &secrets)
		assert.Empty(t, secrets)
	})

	t.Run("CreateInvalidNames", func(t *testing.T) {
		cases := []struct {
			name           string
			expectedStatus int
		}{
			{"-", http.StatusBadRequest},
			{"2secret", http.StatusBadRequest},
			{"GITEA_token", http.StatusBadRequest},
			{"GITHUB_token", http.StatusBadRequest},
		}
		for _, c := range cases {
			req := NewRequestWithJSON(t, "PUT",
				fmt.Sprintf("%s/%s", secretsURL, c.name),
				api.CreateOrUpdateEnvironmentSecretOption{Data: "val"},
			).AddTokenAuth(token)
			MakeRequest(t, req, c.expectedStatus)
		}
	})

	t.Run("CreateAndUpdate", func(t *testing.T) {
		url := secretsURL + "/MY_SECRET"

		req := NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateEnvironmentSecretOption{Data: "initial", Description: "desc"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// list should show it
		req = NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var secrets []*api.EnvironmentSecret
		DecodeJSON(t, resp, &secrets)
		require.Len(t, secrets, 1)
		assert.Equal(t, "MY_SECRET", secrets[0].Name)
		assert.Equal(t, "desc", secrets[0].Description)

		// update (same name → 204)
		req = NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateEnvironmentSecretOption{Data: "updated"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("Delete", func(t *testing.T) {
		url := secretsURL + "/DEL_SECRET"

		req := NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateEnvironmentSecretOption{Data: "val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIRepoEnvironmentVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	envURL := fmt.Sprintf("/api/v1/repos/%s/environments", repo.FullName())
	req := NewRequestWithJSON(t, "POST", envURL, api.CreateEnvironmentOption{Name: "var-env"}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var env api.ActionEnvironment
	DecodeJSON(t, resp, &env)
	varsURL := fmt.Sprintf("%s/%s/variables", envURL, env.Name)

	t.Run("ListEmpty", func(t *testing.T) {
		req := NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var vars []*api.EnvironmentVariable
		DecodeJSON(t, resp, &vars)
		assert.Empty(t, vars)
	})

	t.Run("CreateInvalidNames", func(t *testing.T) {
		cases := []struct {
			name           string
			expectedStatus int
		}{
			{"-", http.StatusBadRequest},
			{"123var", http.StatusBadRequest},
			{"gitea_var", http.StatusBadRequest},
			{"github_var", http.StatusBadRequest},
			{"ci", http.StatusBadRequest},
		}
		for _, c := range cases {
			req := NewRequestWithJSON(t, "POST",
				varsURL+"/"+c.name,
				api.CreateEnvironmentVariableOption{Value: "val"},
			).AddTokenAuth(token)
			MakeRequest(t, req, c.expectedStatus)
		}
	})

	t.Run("CreateAndList", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST",
			varsURL+"/APP_URL",
			api.CreateEnvironmentVariableOption{Value: "https://example.com", Description: "app url"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// duplicate name → conflict
		req = NewRequestWithJSON(t, "POST",
			varsURL+"/app_url",
			api.CreateEnvironmentVariableOption{Value: "other"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)

		req = NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var vars []*api.EnvironmentVariable
		DecodeJSON(t, resp, &vars)
		require.Len(t, vars, 1)
		assert.Equal(t, "APP_URL", vars[0].Name)
		assert.Equal(t, "https://example.com", vars[0].Value)
	})

	t.Run("UpdateAndDelete", func(t *testing.T) {
		// create
		req := NewRequestWithJSON(t, "POST",
			varsURL+"/DB_HOST",
			api.CreateEnvironmentVariableOption{Value: "localhost"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		// get list to find the id
		req = NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var vars []*api.EnvironmentVariable
		DecodeJSON(t, resp, &vars)
		var dbVar *api.EnvironmentVariable
		for _, v := range vars {
			if v.Name == "DB_HOST" {
				dbVar = v
				break
			}
		}
		require.NotNil(t, dbVar)

		// update via PUT /{variablename}
		req = NewRequestWithJSON(t, "PUT",
			varsURL+"/DB_HOST",
			api.UpdateEnvironmentVariableOption{Value: "db.internal"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// delete
		req = NewRequest(t, "DELETE", varsURL+"/DB_HOST").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", varsURL+"/DB_HOST").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
