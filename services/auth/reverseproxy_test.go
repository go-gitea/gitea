// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
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

func TestReverseProxyIgnoresBot(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.ReverseProxyAuthUser, "X-WEBAUTH-USER")()
	defer test.MockVariableValue(&setting.ReverseProxyAuthEmail, "X-WEBAUTH-EMAIL")()
	defer test.MockVariableValue(&setting.Service.EnableReverseProxyEmail, true)()
	require.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: 2, Type: user_model.UserTypeBot}, "type"))

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)
	req.Header.Set(setting.ReverseProxyAuthUser, "user2")
	req.Header.Set(setting.ReverseProxyAuthEmail, "user2@example.com")
	user, err := (&ReverseProxy{}).Verify(req, nil, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, user)
}

type releaseCountingStore struct {
	session.Store
	released int
}

func (s *releaseCountingStore) Release() error {
	s.released++
	return s.Store.Release()
}

func TestReverseProxyLastLogin(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.ReverseProxyAuthUser, "X-WEBAUTH-USER")()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.Zero(t, user.LastLoginUnix)

	sess := &releaseCountingStore{Store: session.NewMockMemStore("reverse-proxy-last-login")}
	ctx, resp := contexttest.MockContext(t, "/", contexttest.MockContextOption{SessionStore: sess})
	ctx.Req.Header.Set(setting.ReverseProxyAuthUser, user.Name)
	rp := &ReverseProxy{CreateSession: true}

	_, err := rp.Verify(ctx.Req, resp, ctx, ctx.Session)
	require.NoError(t, err)
	assert.NotZero(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}).LastLoginUnix)
	assert.Equal(t, 1, sess.released)

	user.LastLoginUnix = 1
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "last_login_unix"))

	_, err = rp.Verify(ctx.Req, resp, ctx, ctx.Session)
	require.NoError(t, err)
	assert.EqualValues(t, 1, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID}).LastLoginUnix) // no write without a new session
	assert.Equal(t, 1, sess.released)
}
