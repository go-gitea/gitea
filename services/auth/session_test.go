// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionVerify(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	sess := session.NewMockMemStore("dummy-sid")
	method := &Session{}

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, sess.Set(session.KeyUID, user.ID))
	u, err := method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, user.ID, u.ID)

	verifySessionCookie := func(sess SessionStore) *http.Cookie {
		resp := httptest.NewRecorder()
		_, err := method.Verify(req, resp, nil, sess)
		require.NoError(t, err)
		for _, c := range resp.Result().Cookies() {
			if c.Name == setting.SessionConfig.CookieName {
				return c
			}
		}
		return nil
	}
	assert.Nil(t, verifySessionCookie(sess)) // password sessions keep the browser-session cookie

	require.NoError(t, sess.Set(session.KeySignInMethod, session.SignInMethodOAuth2))
	c := verifySessionCookie(sess)
	require.NotNil(t, c)
	assert.Equal(t, "dummy-sid", c.Value)
	assert.Equal(t, int(setting.SessionConfig.Maxlifetime), c.MaxAge)
	assert.Nil(t, verifySessionCookie(sess)) // throttled

	// a regenerated session ID keeps the data but must get the persistent cookie at once
	regenerated := session.NewMockMemStore("regenerated-sid")
	for _, k := range []string{session.KeyUID, session.KeySignInMethod, keySessionCookieRefreshedSID, keySessionCookieRefreshedUnix} {
		require.NoError(t, regenerated.Set(k, sess.Get(k)))
	}
	c = verifySessionCookie(regenerated)
	require.NotNil(t, c)
	assert.Equal(t, "regenerated-sid", c.Value)

	require.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: user.ID, Type: user_model.UserTypeBot}, "type"))
	u, err = method.Verify(req, nil, nil, sess)
	assert.NoError(t, err)
	assert.Nil(t, u)
}
