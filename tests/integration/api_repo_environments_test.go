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

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s", baseURL, env.Name)).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var got api.ActionEnvironment
		DecodeJSON(t, resp, &got)
		assert.Equal(t, env.Name, got.Name)
		assert.Equal(t, "main", got.ProtectedBranches)

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

		// create scoped secret and variable before delete
		secretsURL := fmt.Sprintf("%s/%s/secrets", baseURL, env.Name)
		varsURL := fmt.Sprintf("%s/%s/variables", baseURL, env.Name)
		req := NewRequestWithJSON(t, "PUT", secretsURL+"/DEL_SECRET",
			api.CreateOrUpdateSecretOption{Data: "val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "POST", varsURL+"/DEL_VAR",
			api.CreateVariableOption{Value: "val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusNotFound)
		_ = resp
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

	envURL := fmt.Sprintf("/api/v1/repos/%s/environments", repo.FullName())
	req := NewRequestWithJSON(t, "POST", envURL, api.CreateEnvironmentOption{Name: "sec-env"}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var env api.ActionEnvironment
	DecodeJSON(t, resp, &env)
	secretsURL := fmt.Sprintf("%s/%s/secrets", envURL, env.Name)
	repoSecretsURL := fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())

	t.Run("ListEmpty", func(t *testing.T) {
		req := NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var secrets []*api.Secret
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
				api.CreateOrUpdateSecretOption{Data: "val"},
			).AddTokenAuth(token)
			MakeRequest(t, req, c.expectedStatus)
		}
	})

	t.Run("CreateAndUpdate", func(t *testing.T) {
		url := secretsURL + "/MY_SECRET"

		req := NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateSecretOption{Data: "initial", Description: "desc"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var secrets []*api.Secret
		DecodeJSON(t, resp, &secrets)
		require.Len(t, secrets, 1)
		assert.Equal(t, "MY_SECRET", secrets[0].Name)
		assert.Equal(t, "desc", secrets[0].Description)

		req = NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateSecretOption{Data: "updated"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("Delete", func(t *testing.T) {
		url := secretsURL + "/DEL_SECRET"

		req := NewRequestWithJSON(t, "PUT", url,
			api.CreateOrUpdateSecretOption{Data: "val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("RepoSecretsExcludeEnvironmentScoped", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", secretsURL+"/ENV_ONLY",
			api.CreateOrUpdateSecretOption{Data: "env-val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", repoSecretsURL+"/REPO_ONLY",
			api.CreateOrUpdateSecretOption{Data: "repo-val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", repoSecretsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repoSecrets []*api.Secret
		DecodeJSON(t, resp, &repoSecrets)
		repoNames := make([]string, len(repoSecrets))
		for i, s := range repoSecrets {
			repoNames[i] = s.Name
		}
		assert.Contains(t, repoNames, "REPO_ONLY")
		assert.NotContains(t, repoNames, "ENV_ONLY")

		req = NewRequest(t, "GET", secretsURL).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		var envSecrets []*api.Secret
		DecodeJSON(t, resp, &envSecrets)
		envNames := make([]string, len(envSecrets))
		for i, s := range envSecrets {
			envNames[i] = s.Name
		}
		assert.Contains(t, envNames, "ENV_ONLY")
		assert.NotContains(t, envNames, "REPO_ONLY")
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
	repoVarsURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables", repo.FullName())

	t.Run("ListEmpty", func(t *testing.T) {
		req := NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var vars []*api.ActionVariable
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
				api.CreateVariableOption{Value: "val"},
			).AddTokenAuth(token)
			MakeRequest(t, req, c.expectedStatus)
		}
	})

	t.Run("CreateAndList", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST",
			varsURL+"/APP_URL",
			api.CreateVariableOption{Value: "https://example.com", Description: "app url"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "POST",
			varsURL+"/app_url",
			api.CreateVariableOption{Value: "other"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)

		req = NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var vars []*api.ActionVariable
		DecodeJSON(t, resp, &vars)
		require.Len(t, vars, 1)
		assert.Equal(t, "APP_URL", vars[0].Name)
		assert.Equal(t, "https://example.com", vars[0].Data)
	})

	t.Run("UpdateAndDelete", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST",
			varsURL+"/DB_HOST",
			api.CreateVariableOption{Value: "localhost"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT",
			varsURL+"/DB_HOST",
			api.UpdateVariableOption{Value: "db.internal"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", varsURL+"/DB_HOST").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", varsURL+"/DB_HOST").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("RepoVariablesExcludeEnvironmentScoped", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST",
			varsURL+"/ENV_ONLY",
			api.CreateVariableOption{Value: "env-val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "POST",
			repoVarsURL+"/REPO_ONLY",
			api.CreateVariableOption{Value: "repo-val"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", repoVarsURL).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repoVars []*api.ActionVariable
		DecodeJSON(t, resp, &repoVars)
		repoNames := make([]string, len(repoVars))
		for i, v := range repoVars {
			repoNames[i] = v.Name
		}
		assert.Contains(t, repoNames, "REPO_ONLY")
		assert.NotContains(t, repoNames, "ENV_ONLY")

		req = NewRequest(t, "GET", varsURL).AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		var envVars []*api.ActionVariable
		DecodeJSON(t, resp, &envVars)
		envNames := make([]string, len(envVars))
		for i, v := range envVars {
			envNames[i] = v.Name
		}
		assert.Contains(t, envNames, "ENV_ONLY")
		assert.NotContains(t, envNames, "REPO_ONLY")
	})
}
