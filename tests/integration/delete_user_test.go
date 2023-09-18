// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/contexttest"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func assertUserDeleted(t *testing.T, userID int64) {
	unittest.AssertNotExistsBean(t, &user_model.User{ID: userID})
	unittest.AssertNotExistsBean(t, &user_model.Follow{UserID: userID})
	unittest.AssertNotExistsBean(t, &user_model.Follow{FollowID: userID})
	unittest.AssertNotExistsBean(t, &repo_model.Repository{OwnerID: userID})
	unittest.AssertNotExistsBean(t, &access_model.Access{UserID: userID})
	unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: userID})
	unittest.AssertNotExistsBean(t, &issues_model.IssueUser{UID: userID})
	unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: userID})
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: userID})
}

func TestUserDeleteAccount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user8")
	csrf := GetCSRF(t, session, "/user/settings/account")
	urlStr := fmt.Sprintf("/user/settings/account/delete?password=%s", userPassword)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	assertUserDeleted(t, 8)
	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUserDeleteAccountStillOwnRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	csrf := GetCSRF(t, session, "/user/settings/account")
	urlStr := fmt.Sprintf("/user/settings/account/delete?password=%s", userPassword)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"_csrf": csrf,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	// user should not have been deleted, because the user still owns repos
	unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
}

func TestUserDeleteAccountWithWrongPassword(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user8")
	csrf := GetCSRF(t, session, "/user/settings/account")
	urlStr := fmt.Sprintf("/user/settings/account/delete?password=%s", "wrongpassword")
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"_csrf": csrf,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	flashErrMsg := htmlDoc.doc.Find("#ui negative message flash-message flash-error").Text()
	ctx, _ := contexttest.MockContext(t, "/usr/settings/accout")
	assert.Equal(t, ctx.Tr("form.enterred_invalid_password"), flashErrMsg)
}
