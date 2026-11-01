// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"
)

func TestAPIUpdateUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	req := NewRequestWithJSON(t, "PATCH", "/api/v1/user/settings", map[string]string{
		"website": "https://gitea.com",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	t.Run("FullNameLocked", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Admin.UserDisabledFeatures, container.SetOf(setting.UserFeatureChangeFullName))()
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/user/settings", map[string]string{"full_name": "new name"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
