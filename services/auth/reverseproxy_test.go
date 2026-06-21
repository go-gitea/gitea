// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyAuth_BotIgnored(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	bot := &user_model.User{
		Name:               "rp-bot",
		Email:              "rp-bot@example.com",
		Type:               user_model.UserTypeBot,
		MustChangePassword: false,
		IsActive:           true,
	}
	require.NoError(t, user_model.AdminCreateUser(t.Context(), bot, &user_model.Meta{}))

	defer test.MockVariableValue(&setting.Service.EnableReverseProxyEmail, true)()

	rp := &ReverseProxy{}

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	// resolving a bot by reverse-proxy username header must yield no user
	req.Header.Set(setting.ReverseProxyAuthUser, "rp-bot")
	u, err := rp.getUserFromAuthUser(req)
	assert.NoError(t, err)
	assert.Nil(t, u)

	// resolving a bot by reverse-proxy email header must yield no user
	req.Header.Del(setting.ReverseProxyAuthUser)
	req.Header.Set(setting.ReverseProxyAuthEmail, "rp-bot@example.com")
	assert.Nil(t, rp.getUserFromAuthEmail(req))
}
