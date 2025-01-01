// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
	req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	for _, user := range results.Data {
		assert.Contains(t, user.UserName, query)
		assert.NotEmpty(t, user.Email)
	}

	publicToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
		AddTokenAuth(publicToken)
	resp = MakeRequest(t, req, http.StatusOK)
	results = SearchResults{}
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	for _, user := range results.Data {
		assert.Contains(t, user.UserName, query)
		assert.NotEmpty(t, user.Email)
		assert.Equal(t, "public", user.Visibility)
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
		assert.EqualValues(t, modelUser.GetPlaceholderEmail(), user.Email)
	}
}

func TestAPIUserSearchSystemUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	for _, systemUser := range []*user_model.User{
		user_model.NewGhostUser(),
		user_model.NewActionsUser(),
	} {
		t.Run(systemUser.Name, func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/users/search?uid=%d", systemUser.ID)
			resp := MakeRequest(t, req, http.StatusOK)

			var results SearchResults
			DecodeJSON(t, resp, &results)
			assert.NotEmpty(t, results.Data)
			if assert.Len(t, results.Data, 1) {
				user := results.Data[0]
				assert.EqualValues(t, user.UserName, systemUser.Name)
				assert.EqualValues(t, user.ID, systemUser.ID)
			}
		})
	}
}

func TestAPIUserSearchAdminLoggedInUserHidden(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	query := "user31"
	req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
		AddTokenAuth(token)
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

func TestAPIUserSearchByEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// admin can search user with private email
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	query := "user2@example.com"
	req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var results SearchResults
	DecodeJSON(t, resp, &results)
	assert.Len(t, results.Data, 1)
	assert.Equal(t, query, results.Data[0].Email)

	// no login user can not search user with private email
	req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.Empty(t, results.Data)

	// user can search self with private email
	user2 := "user2"
	session = loginUser(t, user2)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &results)
	assert.Len(t, results.Data, 1)
	assert.Equal(t, query, results.Data[0].Email)
}
