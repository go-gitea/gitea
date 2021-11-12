// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPIReposGitNotes(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		user := db.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
		// Login as User2.
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session)

		// check invalid requests
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/12345?token=%s", user.Name, token)
		session.MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/..?token=%s", user.Name, token)
		session.MakeRequest(t, req, http.StatusUnprocessableEntity)

		// check valid request
		req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/notes/65f1bf27bc3bf70f64657658635e66094edbcb4d?token=%s", user.Name, token)
		resp := session.MakeRequest(t, req, http.StatusOK)

		var apiData api.Note
		DecodeJSON(t, resp, &apiData)
		assert.Equal(t, "This is a test note\n", apiData.Message)
	})
}
