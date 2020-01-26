// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
)

func assertUserDeleted(t *testing.T, userID int64) {
	models.AssertNotExistsBean(t, &models.User{ID: userID})
	models.AssertNotExistsBean(t, &models.Follow{UserID: userID})
	models.AssertNotExistsBean(t, &models.Follow{FollowID: userID})
	models.AssertNotExistsBean(t, &models.Repository{OwnerID: userID})
	models.AssertNotExistsBean(t, &models.Access{UserID: userID})
	models.AssertNotExistsBean(t, &models.OrgUser{UID: userID})
	models.AssertNotExistsBean(t, &models.IssueUser{UID: userID})
	models.AssertNotExistsBean(t, &models.TeamUser{UID: userID})
	models.AssertNotExistsBean(t, &models.Star{UID: userID})
}

func TestAdminDeleteUser(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user1")

	csrf := GetCSRF(t, session, "/admin/users/8")
	req := NewRequestWithValues(t, "POST", "/admin/users/8/delete", map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusOK)

	assertUserDeleted(t, 8)
	models.CheckConsistencyFor(t, &models.User{})
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
	models.CheckConsistencyFor(t, &models.User{})
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
	models.AssertExistsAndLoadBean(t, &models.User{ID: 2})
}
