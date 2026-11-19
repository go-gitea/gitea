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
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"
)

func TestAPIRepoSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	token := getTokenForLoggedInUser(t, loginRepoAdmin(t, repo), auth_model.AccessTokenScopeWriteRepository)
	secretsURL := fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())
	putSecret := func(name string, expectedStatus int) {
		option := api.CreateOrUpdateSecretOption{Data: "data", Description: "description"}
		MakeRequest(t, NewRequestWithJSON(t, "PUT", secretsURL+"/"+name, option).AddTokenAuth(token), expectedStatus)
	}

	MakeRequest(t, NewRequest(t, "GET", secretsURL).AddTokenAuth(token), http.StatusOK)
	putSecret("", http.StatusMethodNotAllowed)
	putSecret("-", http.StatusBadRequest)
	putSecret("secret", http.StatusCreated)
	putSecret("secret", http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "DELETE", secretsURL+"/secret").AddTokenAuth(token), http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "DELETE", secretsURL+"/secret").AddTokenAuth(token), http.StatusNotFound)
}
