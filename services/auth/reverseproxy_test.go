// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyLastLogin(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.ReverseProxyAuthUser, "X-WEBAUTH-USER")()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.Zero(t, user.LastLoginUnix)

	ctx, resp := contexttest.MockContext(t, "/", contexttest.MockContextOption{SessionStore: session.NewMockMemStore("reverse-proxy-last-login")})
	ctx.Req.Header.Set(setting.ReverseProxyAuthUser, user.Name)
	rp := &ReverseProxy{CreateSession: true}

	_, err := rp.Verify(ctx.Req, resp, ctx, ctx.Session)
	require.NoError(t, err)
	assert.NotZero(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}).LastLoginUnix)

	user.LastLoginUnix = 1
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "last_login_unix"))

	_, err = rp.Verify(ctx.Req, resp, ctx, ctx.Session)
	require.NoError(t, err)
	assert.EqualValues(t, 1, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}).LastLoginUnix) // no write without a new session
}
