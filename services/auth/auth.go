// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"

	user_model "codenest.dev/models/user"
	"codenest.dev/modules/auth/webauthn"
	"codenest.dev/modules/log"
	"codenest.dev/modules/optional"
	"codenest.dev/modules/session"
	"codenest.dev/modules/web/middleware"
	user_service "codenest.dev/services/user"
)

type ErrUserAuthMessage string

func (e ErrUserAuthMessage) Error() string {
	return string(e)
}

func ErrAsUserAuthMessage(err error) (string, bool) {
	if msg, ok := errors.AsType[ErrUserAuthMessage](err); ok {
		return msg.Error(), true
	}
	return "", false
}

// Init should be called exactly once when the application starts to allow plugins
// to allocate necessary resources
func Init() {
	webauthn.Init()
}

// handleSignInNonInteractive clears existing session variables and stores new ones for the specified user object
// it is mainly for middleware sign-in which doesn't need user's interaction.
func handleSignInNonInteractive(resp http.ResponseWriter, req *http.Request, sess SessionStore, user *user_model.User) {
	sess.Regenerate(resp, req)
	ClearSessionKeysForSignIn(sess)
	if err := sess.Set(session.KeyUID, user.ID); err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}
	if err := sess.Release(); err != nil { // save before a long-lived handler like a websocket runs
		log.Error("Error saving session: %v", err)
	}

	opts := &user_service.UpdateOptions{SetLastLogin: true}
	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(user.Language) == 0 {
		opts.Language = optional.Some(middleware.Locale(resp, req).Language())
	}
	if err := user_service.UpdateUser(req.Context(), user, opts); err != nil {
		log.Error("Error updating user on sign-in [user: %d]: %v", user.ID, err)
	}

	middleware.SetLocaleCookie(resp, user.Language, 0)
}
