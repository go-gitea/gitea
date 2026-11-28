// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/session"
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

	gen, _ := sess.Get(session.KeySessionGeneration).(int64)
	if gen != user.SessionGeneration {
		log.Trace("Session Authorization: Expired session generation for user %-v (got %d, want %d)", user, gen, user.SessionGeneration)
		return nil, nil //nolint:nilnil // the auth method is not applicable
	}

	if rawImpersonator := sess.Get(session.KeyImpersonatorData); rawImpersonator != nil {
		impersonatorData, ok := rawImpersonator.(map[string]any)
		if !ok {
			return nil, nil //nolint:nilnil // the auth method is not applicable
		}
		impersonatorUID, ok := impersonatorData[session.KeyUID].(int64)
		if !ok {
			return nil, nil //nolint:nilnil // the auth method is not applicable
		}
		impersonator, err := user_model.GetUserByID(req.Context(), impersonatorUID)
		if err != nil {
			if !user_model.IsErrUserNotExist(err) {
				log.Error("GetUserByID: %v", err)
				return nil, err
			}
			return nil, nil //nolint:nilnil // the auth method is not applicable
		}
		if !impersonator.IsIndividual() {
			log.Trace("Session Authorization: impersonator %-v is not an individual, ignoring the session", impersonator)
			return nil, nil //nolint:nilnil // the auth method is not applicable
		}
		impersonatorGen, _ := impersonatorData[session.KeySessionGeneration].(int64)
		if impersonatorGen != impersonator.SessionGeneration {
			log.Trace("Session Authorization: Expired session generation for impersonator %-v (got %d, want %d)", impersonator, impersonatorGen, impersonator.SessionGeneration)
			return nil, nil //nolint:nilnil // the auth method is not applicable
		}
	}

	log.Trace("Session Authorization: Logged in user %-v", user)
	return user, nil
}

func ClearSessionKeysForSignIn(sess SessionStore) {
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("webauthnAssertion")
	_ = sess.Delete("linkAccount")
	_ = sess.Delete("linkAccountData")
	_ = sess.Delete(session.KeyImpersonatorData)
	_ = sess.Delete(session.KeyUserHasTwoFactorAuth)
}
