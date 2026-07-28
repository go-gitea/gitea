// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/auth/webauthn"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/session"
	"gitea.dev/modules/web/middleware"
	user_service "gitea.dev/services/user"
)

type ErrUserAuthMessage string

func (e ErrUserAuthMessage) Error() string {
	return string(e)
}

func ErrAsUserAuthMessage(err error) (string, bool) {
	var msg ErrUserAuthMessage
	if errors.As(err, &msg) {
		return msg.Error(), true
	}
	return "", false
}

// Init should be called exactly once when the application starts to allow plugins
// to allocate necessary resources
func Init() {
	webauthn.Init()
}

// handleSignIn clears existing session variables and stores new ones for the specified user object.
// It is only called when establishing a new session (not on every authenticated request).
func handleSignIn(resp http.ResponseWriter, req *http.Request, sess SessionStore, user *user_model.User) {
	// We need to regenerate the session...
	newSess, err := session.RegenerateSession(resp, req)
	if err != nil {
		log.Error(fmt.Sprintf("Error regenerating session: %v", err))
	} else {
		sess = newSess
	}

	ClearSessionKeysForSignIn(sess)
	err = sess.Set(session.KeyUID, user.ID)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}

	// Single UpdateUser: optional language seed + last login (password/OAuth path also sets last login on sign-in).
	opts := &user_service.UpdateOptions{SetLastLogin: true}
	if len(user.Language) == 0 {
		// If the user does not have a locale set, persist the current request locale.
		opts.Language = optional.Some(middleware.Locale(resp, req).Language())
	}
	if err := user_service.UpdateUser(req.Context(), user, opts); err != nil {
		log.Error("Error updating user on sign-in [user: %d]: %v", user.ID, err)
	}

	middleware.SetLocaleCookie(resp, user.Language, 0)
}
