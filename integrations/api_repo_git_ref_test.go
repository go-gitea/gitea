// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
)

func TestAPIReposGitRefs(t *testing.T) {
	defer prepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	for _, ref := range [...]string{
		"refs/heads/master", // Branch
		"refs/tags/v1.1",    // Tag
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/%s?token="+token, user.Name, ref)
		session.MakeRequest(t, req, http.StatusOK)
	}
	// Test getting all refs
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs?token="+token, user.Name)
	session.MakeRequest(t, req, http.StatusOK)
	// Test getting non-existent refs
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/git/refs/heads/unknown?token="+token, user.Name)
	session.MakeRequest(t, req, http.StatusNotFound)
}
