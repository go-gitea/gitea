// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPITeamUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user2Session := loginUser(t, "user2")
	user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("User2ReadUser1", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/teams/1/members/user1").AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("User2ReadSelf", func(t *testing.T) {
		// read self user
		req := NewRequest(t, "GET", "/api/v1/teams/1/members/user2").AddTokenAuth(user2Token)
		resp := MakeRequest(t, req, http.StatusOK)
		var user2 *api.User
		DecodeJSON(t, resp, &user2)
		user2.Created = user2.Created.In(time.Local)
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

		expectedUser := convert.ToUser(t.Context(), user, user)

		// test time via unix timestamp
		assert.Equal(t, expectedUser.LastLogin.Unix(), user2.LastLogin.Unix())
		assert.Equal(t, expectedUser.Created.Unix(), user2.Created.Unix())
		expectedUser.LastLogin = user2.LastLogin
		expectedUser.Created = user2.Created

		assert.Equal(t, expectedUser, user2)
	})
}
