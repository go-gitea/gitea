// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
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

func TestAPIUserInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	user2 := "user31"

	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user3"})

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

		// test if the placaholder Mail is returned if a User is not logged in
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s", user3.Name))
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user3.GetPlaceholderEmail(), u.Email)

		// Test if the correct Mail is returned if a User is logged in
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s?token=%s", user3.Name, token))
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &u)
		assert.Equal(t, user3.GetEmail(), u.Email)
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

func TestAPIUserPackageCount(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	userUrl := "api/v1/users/" + user.Name
	req := NewRequest(t, "GET", userUrl)
	resp := MakeRequest(t, req, http.StatusOK)

	var userInfo *api.User
	DecodeJSON(t, resp, &userInfo)

	assert.Equal(t, 0, userInfo.Packages)

	packageName := "test-package"
	packageVersion := "1.0.3"
	filename := "file.bin"

	url := fmt.Sprintf("/api/packages/%s/generic/%s/%s/%s", user.Name, packageName, packageVersion, filename)
	req = NewRequestWithBody(t, "PUT", url, bytes.NewReader([]byte{}))
	AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "GET", userUrl)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &userInfo)

	assert.Equal(t, 1, userInfo.Packages)
}
