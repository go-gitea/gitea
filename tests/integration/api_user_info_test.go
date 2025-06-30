// Copyright 2022 The Gitea Authors. All rights reserved.
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

func TestAPIUserInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	user2 := "user31"

	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "org3"})

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	t.Run("GetInfo", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/users/"+user2).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user2, u.UserName)

		req = NewRequest(t, "GET", "/api/v1/users/"+user2)
		MakeRequest(t, req, http.StatusNotFound)

		// test if the placaholder Mail is returned if a User is not logged in
		req = NewRequest(t, "GET", "/api/v1/users/"+org3.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &u)
		assert.Equal(t, org3.GetPlaceholderEmail(), u.Email)

		// Test if the correct Mail is returned if a User is logged in
		req = NewRequest(t, "GET", "/api/v1/users/"+org3.Name).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &u)
		assert.Equal(t, org3.GetEmail(), u.Email)
	})

	t.Run("GetAuthenticatedUser", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user, u.UserName)
	})
}
