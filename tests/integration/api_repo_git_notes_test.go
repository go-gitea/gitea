// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposGitNotes(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		// Login as User2.
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		// check invalid requests
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/12345?token=%s", user.Name, token)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/..?token=%s", user.Name, token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)

		// check valid request
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/65f1bf27bc3bf70f64657658635e66094edbcb4d?token=%s", user.Name, token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiData api.Note
		DecodeJSON(t, resp, &apiData)
		assert.Equal(t, "This is a test note\n", apiData.Message)
	})
}
