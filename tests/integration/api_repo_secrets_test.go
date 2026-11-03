// Copyright 2023 The Gitea Authors. All rights reserved.
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

	type secretCase struct {
		Name, Description string
		ExpectedStatus    int
	}
	for _, group := range []struct {
		name  string
		cases []secretCase
	}{
		{"Create", []secretCase{
			{"", "", http.StatusMethodNotAllowed},
			{"-", "", http.StatusBadRequest},
			{"_", "", http.StatusCreated},
			{"secret", "", http.StatusCreated},
			{"2secret", "", http.StatusBadRequest},
			{"GITEA_secret", "", http.StatusBadRequest},
			{"GITHUB_secret", "", http.StatusBadRequest},
		}},
		{"CreateWithDescription", []secretCase{
			{"no_description", "", http.StatusCreated},
			{"description", "some description", http.StatusCreated},
		}},
	} {
		t.Run(group.name, func(t *testing.T) {
			for _, c := range group.cases {
				req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), c.Name), api.CreateOrUpdateSecretOption{
					Data:        "data",
					Description: c.Description,
				}).AddTokenAuth(token)
				MakeRequest(t, req, c.ExpectedStatus)
			}
		})
	}

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
	})
}
