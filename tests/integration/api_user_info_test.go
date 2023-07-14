// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	user2 := "user31"

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	t.Run("GetInfo", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s?token=%s", user2, token))
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user2, u.UserName)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s", user2))
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("GetAuthenticatedUser", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user?token=%s", token))
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user, u.UserName)
	})
}
