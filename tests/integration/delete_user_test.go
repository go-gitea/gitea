// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

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
	urlStr := "/user/settings/account/delete?password=" + userPassword
	req := NewRequest(t, "POST", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.NotEmpty(t, test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)

	assertUserDeleted(t, 8)
	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUserDeleteAccountStillOwnRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	urlStr := "/user/settings/account/delete?password=" + userPassword
	req := NewRequest(t, "POST", urlStr)
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.NotEmpty(t, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage)
	// user should not have been deleted, because the user still owns repos
	unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
}
