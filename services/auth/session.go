// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
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
		return nil, nil
	}

	// Get user ID
	uid := sess.Get("uid")
	if uid == nil {
		return nil, nil
	}
	log.Trace("Session Authorization: Found user[%d]", uid)

	id, ok := uid.(int64)
	if !ok {
		return nil, nil
	}

	// Get user object
	user, err := user_model.GetUserByID(req.Context(), id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByID: %v", err)
			// Return the err as-is to keep current signed-in session, in case the err is something like context.Canceled. Otherwise non-existing user (nil, nil) will make the caller clear the signed-in session.
			return nil, err
		}
		return nil, nil
	}

	log.Trace("Session Authorization: Logged in user %-v", user)
	return user, nil
}
