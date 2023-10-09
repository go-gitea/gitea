// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// GetSessionForLTACookie returns a new session with only the LTA cookie being set.
func GetSessionForLTACookie(t *testing.T, ltaCookie *http.Cookie) *TestSession {
	t.Helper()

	ch := http.Header{}
	ch.Add("Cookie", ltaCookie.String())
	cr := http.Request{Header: ch}

	session := emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	return session
}

// GetLTACookieValue returns the value of the LTA cookie.
func GetLTACookieValue(t *testing.T, sess *TestSession) string {
	t.Helper()

	rememberCookie := sess.GetCookie(setting.CookieRememberName)
	assert.NotNil(t, rememberCookie)

	cookieValue, err := url.QueryUnescape(rememberCookie.Value)
	assert.NoError(t, err)

	return cookieValue
}

// TestSessionCookie checks if the session cookie provides authentication.
func TestSessionCookie(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	sess := loginUser(t, "user1")
	assert.NotNil(t, sess.GetCookie(setting.SessionConfig.CookieName))

	req := NewRequest(t, "GET", "/user/settings")
	sess.MakeRequest(t, req, http.StatusOK)
}

// TestLTACookie checks if the LTA cookie that's returned is valid, exists in the database
// and provides authentication of no session cookie is present.
func TestLTACookie(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	sess := emptyTestSession(t)

	req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"_csrf":     GetCSRF(t, sess, "/user/login"),
		"user_name": user.Name,
		"password":  userPassword,
		"remember":  "true",
	})
	sess.MakeRequest(t, req, http.StatusSeeOther)

	// Checks if the database entry exist for the user.
	ltaCookieValue := GetLTACookieValue(t, sess)
	lookupKey, validator, found := strings.Cut(ltaCookieValue, ":")
	assert.True(t, found)
	rawValidator, err := hex.DecodeString(validator)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &auth.AuthorizationToken{LookupKey: lookupKey, HashedValidator: auth.HashValidator(rawValidator), UID: user.ID})

	// Check if the LTA cookie it provides authentication.
	// If LTA cookie provides authentication /user/login shouldn't return status 200.
	session := GetSessionForLTACookie(t, sess.GetCookie(setting.CookieRememberName))
	req = NewRequest(t, "GET", "/user/login")
	session.MakeRequest(t, req, http.StatusSeeOther)
}

// TestLTAPasswordChange checks that LTA doesn't provide authentication when a
// password change has happened and that the new LTA does provide authentication.
func TestLTAPasswordChange(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	sess := loginUserWithPasswordRemember(t, user.Name, userPassword, true)
	oldRememberCookie := sess.GetCookie(setting.CookieRememberName)
	assert.NotNil(t, oldRememberCookie)

	// Make a simple password change.
	req := NewRequestWithValues(t, "POST", "/user/settings/account", map[string]string{
		"_csrf":        GetCSRF(t, sess, "/user/settings/account"),
		"old_password": userPassword,
		"password":     "password2",
		"retype":       "password2",
	})
	sess.MakeRequest(t, req, http.StatusSeeOther)
	rememberCookie := sess.GetCookie(setting.CookieRememberName)
	assert.NotNil(t, rememberCookie)

	// Check if the password really changed.
	assert.NotEqualValues(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).Passwd, user.Passwd)

	// /user/settings/account should provide with a new LTA cookie, so check for that.
	// If LTA cookie provides authentication /user/login shouldn't return status 200.
	session := GetSessionForLTACookie(t, rememberCookie)
	req = NewRequest(t, "GET", "/user/login")
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Check if the old LTA token is invalidated.
	session = GetSessionForLTACookie(t, oldRememberCookie)
	req = NewRequest(t, "GET", "/user/login")
	session.MakeRequest(t, req, http.StatusOK)
}

// TestLTAExpiry tests that the LTA expiry works.
func TestLTAExpiry(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	sess := loginUserWithPasswordRemember(t, user.Name, userPassword, true)

	ltaCookieValie := GetLTACookieValue(t, sess)
	lookupKey, _, found := strings.Cut(ltaCookieValie, ":")
	assert.True(t, found)

	// Ensure it's not expired.
	lta := unittest.AssertExistsAndLoadBean(t, &auth.AuthorizationToken{UID: user.ID, LookupKey: lookupKey})
	assert.False(t, lta.IsExpired())

	// Manually stub LTA's expiry.
	_, err := db.GetEngine(db.DefaultContext).ID(lta.ID).Table("authorization_token").Cols("expiry").Update(&auth.AuthorizationToken{Expiry: timeutil.TimeStampNow()})
	assert.NoError(t, err)

	// Ensure it's expired.
	lta = unittest.AssertExistsAndLoadBean(t, &auth.AuthorizationToken{UID: user.ID, LookupKey: lookupKey})
	assert.True(t, lta.IsExpired())

	// Should return 200 OK, because LTA doesn't provide authorization anymore.
	session := GetSessionForLTACookie(t, sess.GetCookie(setting.CookieRememberName))
	req := NewRequest(t, "GET", "/user/login")
	session.MakeRequest(t, req, http.StatusOK)

	// Ensure it's deleted.
	unittest.AssertNotExistsBean(t, &auth.AuthorizationToken{UID: user.ID, LookupKey: lookupKey})
}
