// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIReposRaw(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	for _, ref := range [...]string{
		"master", // Branch
		"v1.1",   // Tag
		"65f1bf27bc3bf70f64657658635e66094edbcb4d", // Commit
	} {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/raw/%s/README.md?token="+token, user.Name, ref)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.EqualValues(t, "file", resp.Header().Get("x-gitea-object-type"))
	}
	// Test default branch
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/raw/README.md?token="+token, user.Name)
	resp := MakeRequest(t, req, http.StatusOK)
	assert.EqualValues(t, "file", resp.Header().Get("x-gitea-object-type"))
}
