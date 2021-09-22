// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
)

func assertUserDeleted(t *testing.T, userID int64) {
	db.AssertNotExistsBean(t, &models.User{ID: userID})
	db.AssertNotExistsBean(t, &models.Follow{UserID: userID})
	db.AssertNotExistsBean(t, &models.Follow{FollowID: userID})
	db.AssertNotExistsBean(t, &models.Repository{OwnerID: userID})
	db.AssertNotExistsBean(t, &models.Access{UserID: userID})
	db.AssertNotExistsBean(t, &models.OrgUser{UID: userID})
	db.AssertNotExistsBean(t, &models.IssueUser{UID: userID})
	db.AssertNotExistsBean(t, &models.TeamUser{UID: userID})
	db.AssertNotExistsBean(t, &models.Star{UID: userID})
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
	db.AssertExistsAndLoadBean(t, &models.User{ID: 2})
}
