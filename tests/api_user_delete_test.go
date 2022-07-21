// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

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
