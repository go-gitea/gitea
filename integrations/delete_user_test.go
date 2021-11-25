// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
)

func assertUserDeleted(t *testing.T, userID int64) {
	unittest.AssertNotExistsBean(t, &user_model.User{ID: userID})
	unittest.AssertNotExistsBean(t, &user_model.Follow{UserID: userID})
	unittest.AssertNotExistsBean(t, &user_model.Follow{FollowID: userID})
	unittest.AssertNotExistsBean(t, &models.Repository{OwnerID: userID})
	unittest.AssertNotExistsBean(t, &models.Access{UserID: userID})
	unittest.AssertNotExistsBean(t, &models.OrgUser{UID: userID})
	unittest.AssertNotExistsBean(t, &models.IssueUser{UID: userID})
	unittest.AssertNotExistsBean(t, &models.TeamUser{UID: userID})
	unittest.AssertNotExistsBean(t, &models.Star{UID: userID})
}

func TestUserDeleteAccount(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user8")
	csrf := GetCSRF(t, session, "/user/settings/account")
	urlStr := fmt.Sprintf("/user/settings/account/delete?password=%s", userPassword)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusFound)

	assertUserDeleted(t, 8)
	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUserDeleteAccountStillOwnRepos(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	csrf := GetCSRF(t, session, "/user/settings/account")
	urlStr := fmt.Sprintf("/user/settings/account/delete?password=%s", userPassword)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusFound)

	// user should not have been deleted, because the user still owns repos
	unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
}
