// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	secret_model "gitea.dev/models/secret"
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
	token := getTokenForLoggedInUser(t, loginUser(t, user.Name), auth_model.AccessTokenScopeWriteRepository)
	baseURL := fmt.Sprintf("/api/v1/repos/%s/environments", repo.FullName())

	t.Run("PutCreatesThenUpdatesInPlace", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", baseURL+"/production", &api.CreateOrUpdateEnvironmentOption{
			AllowedBranchPatterns: []string{"main"},
		}).AddTokenAuth(token)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ActionEnvironment{})
		assert.Equal(t, "production", created.Name)
		assert.Equal(t, []string{"main"}, created.AllowedBranchPatterns)

		// names are case-insensitive, so this updates the existing row and keeps its spelling
		req = NewRequestWithJSON(t, "PUT", baseURL+"/PRODUCTION", &api.CreateOrUpdateEnvironmentOption{
			AllowedBranchPatterns: []string{"main", "refs/tags/v*"},
		}).AddTokenAuth(token)
		updated := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ActionEnvironment{})
		assert.Equal(t, created.ID, updated.ID)
		assert.Equal(t, "production", updated.Name)
		assert.Equal(t, []string{"main", "refs/tags/v*"}, updated.AllowedBranchPatterns)

		// the body replaces the policy, so omitting the patterns allows every ref
		req = NewRequestWithJSON(t, "PUT", baseURL+"/production", &api.CreateOrUpdateEnvironmentOption{}).AddTokenAuth(token)
		cleared := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ActionEnvironment{})
		assert.Equal(t, []string{}, cleared.AllowedBranchPatterns)
	})

	t.Run("RejectsAnInvalidPattern", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", baseURL+"/staging", &api.CreateOrUpdateEnvironmentOption{
			AllowedBranchPatterns: []string{"["},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("OnlyTheOwnerMayUseIt", func(t *testing.T) {
		other := getTokenForLoggedInUser(t, loginUser(t, "user4"), auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestWithJSON(t, "PUT", baseURL+"/production", &api.CreateOrUpdateEnvironmentOption{}).AddTokenAuth(other)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("UnknownEnvironmentIsNotFound", func(t *testing.T) {
		for _, path := range []string{"", "/secrets", "/variables"} {
			MakeRequest(t, NewRequest(t, "GET", baseURL+"/missing"+path).AddTokenAuth(token), http.StatusNotFound)
		}
		req := NewRequestWithJSON(t, "POST", baseURL+"/missing/variables/APP_URL", &api.CreateVariableOption{Value: "x"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("SecretsAndVariablesAreScopedToTheEnvironment", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", baseURL+"/production/secrets/DEPLOY_TOKEN", &api.CreateOrUpdateSecretOption{
			Data: "env-token",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "POST", baseURL+"/production/variables/APP_URL", &api.CreateVariableOption{
			Value: "https://prod.example.com",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "POST", baseURL+"/production/variables/APP_URL", &api.CreateVariableOption{Value: "again"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusConflict)

		req = NewRequestWithJSON(t, "PUT", baseURL+"/production/variables/APP_URL", &api.UpdateVariableOption{
			Value: "https://new.example.com",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		assert.Equal(t, "https://new.example.com",
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionVariable{RepoID: repo.ID, Name: "APP_URL"}).Data)

		req = NewRequest(t, "GET", baseURL+"/production/secrets").AddTokenAuth(token)
		var secrets []*api.Secret
		DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &secrets)
		require.Len(t, secrets, 1)
		assert.Equal(t, "DEPLOY_TOKEN", secrets[0].Name)

		// The repository scope must not see the environment's values.
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())).AddTokenAuth(token)
		var repoSecrets []*api.Secret
		DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &repoSecrets)
		assert.Empty(t, repoSecrets)

		MakeRequest(t, NewRequest(t, "DELETE", baseURL+"/production/variables/APP_URL").AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "DELETE", baseURL+"/production/secrets/DEPLOY_TOKEN").AddTokenAuth(token), http.StatusNoContent)

		// recreated so the cascade below has something to remove
		req = NewRequestWithJSON(t, "PUT", baseURL+"/production/secrets/DEPLOY_TOKEN", &api.CreateOrUpdateSecretOption{Data: "again"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("DeleteCascadesToSecretsAndVariables", func(t *testing.T) {
		env := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionEnvironment{RepoID: repo.ID, LowerName: "production"})

		req := NewRequest(t, "DELETE", baseURL+"/production").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", baseURL+"/production").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)

		unittest.AssertNotExistsBean(t, &secret_model.Secret{EnvironmentID: env.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionVariable{EnvironmentID: env.ID})
	})
}
