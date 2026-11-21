// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateUserSettingsChangeFullNameDisabled(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	WithDisabledFeatures(t, setting.UserFeatureChangeFullName)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	// sending the current full name back is not a change, so it is still allowed
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/user/settings", &api.UserSettingsOptions{
		FullName: &user.FullName,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	newFullName := "New Full Name"
	req = NewRequestWithJSON(t, "PATCH", "/api/v1/user/settings", &api.UserSettingsOptions{
		FullName: &newFullName,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	assert.NotEqual(t, newFullName, user.FullName)
}
