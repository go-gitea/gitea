// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"time"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
)

// Ensure the struct implements the interface.
var (
	_ Method = &Session{}
)

// Session checks if there is a user uid stored in the session and returns the user
// object for that uid.
type Session struct{}

// Name represents the name of auth method
func (s *Session) Name() string {
	return "session"
}

// Verify checks if there is a user uid stored in the session and returns the user
// object for that uid.
// Returns nil if there is no user uid stored in the session.
func (s *Session) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	if sess == nil {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	// Get session user ID
	uid, ok := sess.Get(session.KeyUID).(int64)
	if !ok {
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	// Get user object
	user, err := user_model.GetUserByID(req.Context(), uid)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByID: %v", err)
			// Return the err as-is to keep current signed-in session, in case the err is something like context.Canceled. Otherwise non-existing user (nil, nil) will make the caller clear the signed-in session.
			return nil, err
		}
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	// sessions can't be enumerated per user, so one opened before a conversion to bot is rejected here
	if !user.IsIndividual() {
		log.Trace("Session Authorization: user %-v is not an individual, ignoring the session", user)
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	log.Trace("Session Authorization: Logged in user %-v", user)
	if sess.Get(session.KeySignInMethod) == session.SignInMethodOAuth2 {
		refreshPersistentSessionCookie(w, sess)
	}
	return user, nil
}

const (
	keySessionCookieRefreshedSID  = "sessionCookieRefreshedSID"
	keySessionCookieRefreshedUnix = "sessionCookieRefreshedUnix"
)

// SSO sessions should outlive the browser like the provider's own session, so the cookie expiry slides with activity
func refreshPersistentSessionCookie(w http.ResponseWriter, sess SessionStore) {
	lifetime := time.Duration(setting.SessionConfig.Maxlifetime) * time.Second
	// the data survives a session ID regeneration, but the new ID arrives as a browser-session cookie
	refreshedSID, _ := sess.Get(keySessionCookieRefreshedSID).(string)
	refreshedUnix, _ := sess.Get(keySessionCookieRefreshedUnix).(int64)
	if refreshedSID == sess.ID() && time.Since(time.Unix(refreshedUnix, 0)) < min(lifetime/10, time.Hour) {
		return
	}
	_ = sess.Set(keySessionCookieRefreshedSID, sess.ID())
	_ = sess.Set(keySessionCookieRefreshedUnix, time.Now().Unix())
	http.SetCookie(w, &http.Cookie{
		Name:     setting.SessionConfig.CookieName,
		Value:    sess.ID(),
		Path:     setting.SessionConfig.CookiePath,
		MaxAge:   int(lifetime.Seconds()),
		HttpOnly: true,
		Secure:   setting.SessionConfig.Secure,
		Domain:   setting.SessionConfig.Domain,
		SameSite: setting.SessionConfig.SameSite,
	})
}

func ClearSessionKeysForSignIn(sess SessionStore) {
	_ = sess.Delete("openid_verified_uri")
	_ = sess.Delete("openid_signin_remember")
	_ = sess.Delete("openid_determined_email")
	_ = sess.Delete("openid_determined_username")
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("webauthnAssertion")
	_ = sess.Delete("linkAccount")
	_ = sess.Delete("linkAccountData")
	_ = sess.Delete("openidPendingURI")
}
