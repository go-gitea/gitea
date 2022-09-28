// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"
)

func TestAPIDeleteUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// 1 -> Admin
	// 8 -> Normal user
	for _, userID := range []int64{1, 8} {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
		t.Logf("Testing username %s", user.Name)

		req := NewRequest(t, "DELETE", "/api/v1/user")
		req = AddBasicAuthHeader(req, user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		assertUserDeleted(t, userID)
		unittest.CheckConsistencyFor(t, &user_model.User{})
	}
}

func TestAPIPurgeUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// Cannot delete the user as it still has ownership of repositories
	req := NewRequest(t, "DELETE", "/api/v1/user")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	unittest.CheckConsistencyFor(t, &user_model.User{ID: 5})

	req = NewRequest(t, "DELETE", "/api/v1/user?purge=true")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusNoContent)

	assertUserDeleted(t, 5)
	unittest.CheckConsistencyFor(t, &user_model.User{}, &repo_model.Repository{})
}
