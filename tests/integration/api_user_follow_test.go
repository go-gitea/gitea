// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIFollow(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user1 := "user4"
	user2 := "user1"

	session1 := loginUser(t, user1)
	token1 := getTokenForLoggedInUser(t, session1)

	session2 := loginUser(t, user2)
	token2 := getTokenForLoggedInUser(t, session2)

	t.Run("Follow", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", fmt.Sprintf("/api/v1/user/following/%s?token=%s", user1, token2))
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("ListFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following?token=%s", user2, token2))
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user1, users[0].UserName)
	})

	t.Run("ListMyFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/following?token=%s", token2))
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user1, users[0].UserName)
	})

	t.Run("ListFollowers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/followers?token=%s", user1, token1))
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user2, users[0].UserName)
	})

	t.Run("ListMyFollowers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/followers?token=%s", token1))
		resp := MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.Equal(t, user2, users[0].UserName)
	})

	t.Run("CheckFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following/%s?token=%s", user2, user1, token2))
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/following/%s?token=%s", user1, user2, token2))
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("CheckMyFollowing", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/following/%s?token=%s", user1, token2))
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/following/%s?token=%s", user2, token1))
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Unfollow", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/user/following/%s?token=%s", user1, token2))
		MakeRequest(t, req, http.StatusNoContent)
	})
}
