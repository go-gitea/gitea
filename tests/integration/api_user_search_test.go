// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

type SearchResults struct {
	OK   bool        `json:"ok"`
	Data []*api.User `json:"data"`
}

func TestAPIUserSearchLoggedIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	query := "user2"
	req := NewRequestf(t, "GET", "/api/v1/users/search?token=%s&q=%s", token, query)
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	for _, user := range results.Data {
		assert.Contains(t, user.UserName, query)
		assert.NotEmpty(t, user.Email)
	}
}

func TestAPIUserSearchNotLoggedIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	query := "user2"
	req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	var modelUser *user_model.User
	for _, user := range results.Data {
		assert.Contains(t, user.UserName, query)
		modelUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
		if modelUser.KeepEmailPrivate {
			assert.EqualValues(t, fmt.Sprintf("%s@%s", modelUser.LowerName, setting.Service.NoReplyAddress), user.Email)
		} else {
			assert.EqualValues(t, modelUser.Email, user.Email)
		}
	}
}

func TestAPIUserSearchAdminLoggedInUserHidden(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	query := "user31"
	req := NewRequestf(t, "GET", "/api/v1/users/search?token=%s&q=%s", token, query)
	req.SetBasicAuth(token, "x-oauth-basic")
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	for _, user := range results.Data {
		assert.Contains(t, user.UserName, query)
		assert.NotEmpty(t, user.Email)
		assert.EqualValues(t, "private", user.Visibility)
	}
}

func TestAPIUserSearchNotLoggedInUserHidden(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	query := "user31"
	req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.Empty(t, results.Data)
}
