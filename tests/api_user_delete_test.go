// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
)

func TestAPIDeleteUser(t *testing.T) {
	defer prepareTestEnv(t)()

	// 1 -> Admin
	// 8 -> Normal user
	for _, userID := range []int{1, 8} {
		username := fmt.Sprintf("user%d", userID)
		t.Logf("Testing username %s", username)

		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)

		req := NewRequest(t, "DELETE", "/api/v1/user?token="+token)
		session.MakeRequest(t, req, http.StatusNoContent)

		assertUserDeleted(t, int64(userID))
		unittest.CheckConsistencyFor(t, &user_model.User{})
	}
}

func TestAPIPurgeUser(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user5")
	token := getTokenForLoggedInUser(t, session)

	// Cannot delete the user as it still has ownership of repositories
	req := NewRequest(t, "DELETE", "/api/v1/user?token="+token)
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	unittest.CheckConsistencyFor(t, &user_model.User{ID: 5})

	req = NewRequest(t, "DELETE", "/api/v1/user?purge=true&token="+token)
	session.MakeRequest(t, req, http.StatusNoContent)

	assertUserDeleted(t, 5)
	unittest.CheckConsistencyFor(t, &user_model.User{}, &repo_model.Repository{})
}
