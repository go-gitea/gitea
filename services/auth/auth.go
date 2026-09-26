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
