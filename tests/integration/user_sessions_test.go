// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	"gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSignOutSessions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user4"})
	baseURL, err := url.Parse(setting.AppURL)
	require.NoError(t, err)

	assertSignedIn := func(t *testing.T, session *TestSession, signedIn bool) {
		t.Helper()
		expected := http.StatusSeeOther
		if signedIn {
			expected = http.StatusOK
		}
		session.MakeRequest(t, NewRequest(t, "GET", "/user/settings/security"), expected)
	}

	first := loginUser(t, user.Name)

	second := emptyTestSession(t)
	second.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"user_name": user.Name,
		"password":  userPassword,
		"remember":  "on",
	}), http.StatusSeeOther)
	rememberCookie := second.GetRawCookie(setting.CookieRememberName)
	require.NotNil(t, rememberCookie)

	otherUserSession := loginUser(t, otherUser.Name)

	resp := first.MakeRequest(t, NewRequest(t, "GET", "/user/settings/security"), http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, doc, "button[data-url$='/sessions/sign_out']", true)

	// Sign out all browsers for user2.
	first.MakeRequest(t, NewRequest(t, "POST", "/user/settings/security/sessions/sign_out"), http.StatusOK)

	// Both active sessions of user2 are now signed out.
	assertSignedIn(t, first, false)
	assertSignedIn(t, second, false)
	loginResp := second.MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK)
	AssertHTMLElement(t, NewHTMLParser(t, loginResp.Body), ".flash-error", false)

	// The remember-me token of user2 is also invalidated.
	unittest.AssertNotExistsBean(t, &auth.AuthToken{UserID: user.ID})
	ltaSession := emptyTestSession(t)
	ltaSession.jar.SetCookies(baseURL, []*http.Cookie{rememberCookie})
	ltaSession.MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK)

	// Other users are unaffected.
	assertSignedIn(t, otherUserSession, true)

	// Signing in again works with the bumped session generation.
	relogged := loginUser(t, user.Name)
	assertSignedIn(t, relogged, true)
	updatedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.EqualValues(t, 1, updatedUser.SessionGeneration)
}
