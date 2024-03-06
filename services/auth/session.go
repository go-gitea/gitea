// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
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
	user := SessionUser(req.Context(), sess)
	if user != nil {
		return user, nil
	}
	return nil, nil
}

// SessionUser returns the user object corresponding to the "uid" session variable.
func SessionUser(ctx context.Context, sess SessionStore) *user_model.User {
	if sess == nil {
		return nil
	}

	// Get user ID
	uid := sess.Get("uid")
	if uid == nil {
		return nil
	}
	log.Trace("Session Authorization: Found user[%d]", uid)

	id, ok := uid.(int64)
	if !ok {
		return nil
	}

	// Get user object
	user, err := user_model.GetUserByID(ctx, id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserById: %v", err)
		}
		return nil
	}

	log.Trace("Session Authorization: Logged in user %-v", user)
	return user
}
