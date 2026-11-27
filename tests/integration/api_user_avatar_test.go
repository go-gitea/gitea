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

	t.Run("Delete", func(t *testing.T) {
		req := NewRequest(t, "DELETE", "/api/v1/user/avatar").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("Update", func(t *testing.T) {
		// Test what happens if you use a valid image
		avatar, err := os.ReadFile(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/integration/avatar.png"))
		assert.NoError(t, err)
		if err != nil {
			assert.FailNow(t, "Unable to open avatar.png")
		}

		for _, avatarTest := range []struct {
			image  string
			status int
		}{
			{base64.StdEncoding.EncodeToString(avatar), http.StatusNoContent},
			// Test what happens if you don't have a valid Base64 string
			{"Invalid", http.StatusBadRequest},
			// Test what happens if you use a file that is not an image
			{base64.StdEncoding.EncodeToString([]byte("This is not an image")), http.StatusInternalServerError},
		} {
			req := NewRequestWithJSON(t, "POST", "/api/v1/user/avatar", &api.UpdateUserAvatarOption{
				Image: avatarTest.image,
			}).AddTokenAuth(token)
			MakeRequest(t, req, avatarTest.status)
		}
	})
}
