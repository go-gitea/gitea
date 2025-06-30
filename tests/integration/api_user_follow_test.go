// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIFollow(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user1 := "user4"
	user2 := "user1"

	session1 := loginUser(t, user1)
	token1 := getTokenForLoggedInUser(t, session1, auth_model.AccessTokenScopeReadUser)

	session2 := loginUser(t, user2)
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteUser)

	t.Run("Follow", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", "/api/v1/user/following/"+user1).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		// blocked user can't follow blocker
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		req = NewRequest(t, "PUT", "/api/v1/user/following/user2").
			AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteUser))
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("ListFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following", user2)).
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user1, users[0].UserName)
	})

	t.Run("ListMyFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/following").
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user1, users[0].UserName)
	})

	t.Run("ListFollowers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/followers", user1)).
			AddTokenAuth(token1)
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user2, users[0].UserName)
	})

	t.Run("ListMyFollowers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/followers").
			AddTokenAuth(token1)
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user2, users[0].UserName)
	})

	t.Run("CheckFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following/%s", user2, user1)).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following/%s", user1, user2)).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("CheckMyFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/following/"+user1).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", "/api/v1/user/following/"+user2).
			AddTokenAuth(token1)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Unfollow", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", "/api/v1/user/following/"+user1).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusNoContent)
	})
}
