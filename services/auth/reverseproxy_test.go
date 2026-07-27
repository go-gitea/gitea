// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	session_module "gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyVerifyUpdatesLastLoginOnSessionEstablish(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user.LastLoginUnix = timeutil.TimeStamp(0)
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "last_login_unix"))

	sess := session_module.NewMockMemStore("reverse-proxy-last-login")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		context.WithValue(t.Context(), session_module.MockStoreContextKey, sess),
	)
	req.Header.Set(setting.ReverseProxyAuthUser, user.Name)

	// First request establishes a session and must record last login.
	verifiedUser, err := (&ReverseProxy{CreateSession: true}).Verify(req, httptest.NewRecorder(), reqctx.ContextData{}, sess)
	require.NoError(t, err)
	require.NotNil(t, verifiedUser)

	updatedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.NotZero(t, updatedUser.LastLoginUnix)
	firstLogin := updatedUser.LastLoginUnix

	// Subsequent request with the same session must not rewrite last_login_unix
	// (would be a performance problem if done on every reverse-proxy request).
	// Force a known stale value so we can detect unwanted updates.
	const staleLastLogin = timeutil.TimeStamp(1_000_000)
	updatedUser.LastLoginUnix = staleLastLogin
	require.NoError(t, user_model.UpdateUserCols(t.Context(), updatedUser, "last_login_unix"))

	verifiedUser, err = (&ReverseProxy{CreateSession: true}).Verify(req, httptest.NewRecorder(), reqctx.ContextData{}, sess)
	require.NoError(t, err)
	require.NotNil(t, verifiedUser)

	afterSecond := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.Equal(t, staleLastLogin, afterSecond.LastLoginUnix)
	assert.NotEqual(t, firstLogin, afterSecond.LastLoginUnix) // sanity: we did change it to stale
}

func TestReverseProxyVerifyWithoutCreateSessionDoesNotUpdateLastLogin(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user.LastLoginUnix = timeutil.TimeStamp(0)
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "last_login_unix"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(setting.ReverseProxyAuthUser, user.Name)

	verifiedUser, err := (&ReverseProxy{CreateSession: false}).Verify(req, httptest.NewRecorder(), reqctx.ContextData{}, nil)
	require.NoError(t, err)
	require.NotNil(t, verifiedUser)

	// Sessionless API-style reverse proxy auth must not touch last login.
	after := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.Zero(t, after.LastLoginUnix)
}
