// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserInfo(t *testing.T) {
	defer prepareTestEnv(t)()

	user := "user1"
	user2 := "user31"

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session)

	t.Run("GetInfo", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s?token=%s", user2, token))
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user2, u.UserName)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s", user2))
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("GetAuthenticatedUser", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user?token=%s", token))
		resp := MakeRequest(t, req, http.StatusOK)

		var u api.User
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user, u.UserName)
	})
}
