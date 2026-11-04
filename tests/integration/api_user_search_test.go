// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

type SearchResults struct {
	OK   bool        `json:"ok"`
	Data []*api.User `json:"data"`
}

func TestAPIUserSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("LoggedIn", func(t *testing.T) {
		adminUsername := "user1"
		session := loginUser(t, adminUsername)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
		query := "user2"
		req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		results := DecodeJSON(t, resp, &SearchResults{})
		assert.NotEmpty(t, results.Data)
		for _, user := range results.Data {
			assert.Contains(t, user.UserName, query)
			assert.NotEmpty(t, user.Email)
		}

		publicToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
		req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
			AddTokenAuth(publicToken)
		resp = MakeRequest(t, req, http.StatusOK)
		results = DecodeJSON(t, resp, &SearchResults{})
		assert.NotEmpty(t, results.Data)
		for _, user := range results.Data {
			assert.Contains(t, user.UserName, query)
			assert.NotEmpty(t, user.Email)
			assert.Equal(t, api.VisibilityStringPublic, user.Visibility)
		}
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		query := "user2"
		req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
		resp := MakeRequest(t, req, http.StatusOK)

		results := DecodeJSON(t, resp, &SearchResults{})
		assert.NotEmpty(t, results.Data)
		var modelUser *user_model.User
		for _, user := range results.Data {
			assert.Contains(t, user.UserName, query)
			modelUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
			assert.Equal(t, modelUser.GetPlaceholderEmail(), user.Email)
		}
	})

	t.Run("SystemUsers", func(t *testing.T) {
		for _, systemUser := range []*user_model.User{
			user_model.NewGhostUser(),
			user_model.NewActionsUser(),
		} {
			t.Run(systemUser.Name, func(t *testing.T) {
				req := NewRequestf(t, "GET", "/api/v1/users/search?uid=%d", systemUser.ID)
				resp := MakeRequest(t, req, http.StatusOK)

				results := DecodeJSON(t, resp, &SearchResults{})
				assert.NotEmpty(t, results.Data)
				if assert.Len(t, results.Data, 1) {
					user := results.Data[0]
					assert.Equal(t, user.UserName, systemUser.Name)
					assert.Equal(t, user.ID, systemUser.ID)
				}
			})
		}
	})

	t.Run("AdminLoggedInUserHidden", func(t *testing.T) {
		adminUsername := "user1"
		session := loginUser(t, adminUsername)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
		query := "user31"
		req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		results := DecodeJSON(t, resp, &SearchResults{})
		assert.NotEmpty(t, results.Data)
		for _, user := range results.Data {
			assert.Contains(t, user.UserName, query)
			assert.NotEmpty(t, user.Email)
			assert.Equal(t, api.VisibilityStringPrivate, user.Visibility)
		}
	})

	t.Run("NotLoggedInUserHidden", func(t *testing.T) {
		query := "user31"
		req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
		resp := MakeRequest(t, req, http.StatusOK)

		results := DecodeJSON(t, resp, &SearchResults{})
		assert.Empty(t, results.Data)
	})

	t.Run("ByEmail", func(t *testing.T) {
		// admin can search user with private email
		adminUsername := "user1"
		session := loginUser(t, adminUsername)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
		query := "user2@example.com"
		req := NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		results := DecodeJSON(t, resp, &SearchResults{})
		assert.Len(t, results.Data, 1)
		assert.Equal(t, query, results.Data[0].Email)

		// no login user can not search user with private email
		req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query)
		resp = MakeRequest(t, req, http.StatusOK)
		results = DecodeJSON(t, resp, &SearchResults{})
		assert.Empty(t, results.Data)

		// user can search self with private email
		user2 := "user2"
		session = loginUser(t, user2)
		token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
		req = NewRequestf(t, "GET", "/api/v1/users/search?q=%s", query).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)

		results = DecodeJSON(t, resp, &SearchResults{})
		assert.Len(t, results.Data, 1)
		assert.Equal(t, query, results.Data[0].Email)
	})
}
