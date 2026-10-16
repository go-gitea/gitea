// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateUserAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	// Test what happens if you use a valid image
	avatar, err := os.ReadFile(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/integration/avatar.png"))
	assert.NoError(t, err)
	if err != nil {
		assert.FailNow(t, "Unable to open avatar.png")
	}

	// Test what happens if you don't have a valid Base64 string
	opts := api.UpdateUserAvatarOption{
		Image: base64.StdEncoding.EncodeToString(avatar),
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/avatar", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	opts = api.UpdateUserAvatarOption{
		Image: "Invalid",
	}

	req = NewRequestWithJSON(t, "POST", "/api/v1/user/avatar", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)

	// Test what happens if you use a file that is not an image
	opts = api.UpdateUserAvatarOption{
		Image: base64.StdEncoding.EncodeToString([]byte("This is not an image")),
	}

	req = NewRequestWithJSON(t, "POST", "/api/v1/user/avatar", &opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusInternalServerError)
}

func TestAPIDeleteUserAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	req := NewRequest(t, "DELETE", "/api/v1/user/avatar").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
