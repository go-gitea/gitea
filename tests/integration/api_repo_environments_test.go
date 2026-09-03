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

		// The same request again updates the existing row rather than conflicting.
		req = NewRequestWithJSON(t, "PUT", baseURL+"/production", &api.CreateOrUpdateEnvironmentOption{
			AllowedBranchPatterns: []string{"main", "release/*"},
		}).AddTokenAuth(token)
		updated := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ActionEnvironment{})
		assert.Equal(t, created.ID, updated.ID)
		assert.Equal(t, []string{"main", "release/*"}, updated.AllowedBranchPatterns)
	})

	t.Run("NamesAreCaseInsensitive", func(t *testing.T) {
		req := NewRequest(t, "GET", baseURL+"/PRODUCTION").AddTokenAuth(token)
		got := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ActionEnvironment{})
		assert.Equal(t, "production", got.Name, "the stored spelling is preserved")
	})

	t.Run("RejectsAnUnusableNameAndPattern", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", baseURL+"/bad%2Fname", &api.CreateOrUpdateEnvironmentOption{}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)

		req = NewRequestWithJSON(t, "PUT", baseURL+"/staging", &api.CreateOrUpdateEnvironmentOption{
			AllowedBranchPatterns: []string{"["},
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)
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
