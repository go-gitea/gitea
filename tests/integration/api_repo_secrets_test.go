// Copyright 2023 The Gitea Authors. All rights reserved.
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

func TestAPIRepoSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("List", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("Create", func(t *testing.T) {
		cases := []struct {
			Name           string
			ExpectedStatus int
		}{
			{
				Name:           "",
				ExpectedStatus: http.StatusMethodNotAllowed,
			},
			{
				Name:           "-",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "_",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "secret",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "2secret",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "GITEA_secret",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "GITHUB_secret",
				ExpectedStatus: http.StatusBadRequest,
			},
		}

		for _, c := range cases {
			req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), c.Name), api.CreateOrUpdateSecretOption{
				Data: "data",
			}).AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)
		}
	})

	t.Run("Update", func(t *testing.T) {
		name := "update_secret"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "initial",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "changed",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "delete_secret"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "initial",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/secrets/000", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)
	})
}
