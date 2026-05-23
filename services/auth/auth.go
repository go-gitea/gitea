// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/webauthn"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/web/middleware"
	user_service "code.gitea.io/gitea/services/user"
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

// handleSignIn clears existing session variables and stores new ones for the specified user object
func handleSignIn(resp http.ResponseWriter, req *http.Request, sess SessionStore, user *user_model.User) {
	// We need to regenerate the session...
	newSess, err := session.RegenerateSession(resp, req)
	if err != nil {
		log.Error(fmt.Sprintf("Error regenerating session: %v", err))
	} else {
		sess = newSess
	}

	_ = sess.Delete("openid_verified_uri")
	_ = sess.Delete("openid_signin_remember")
	_ = sess.Delete("openid_determined_email")
	_ = sess.Delete("openid_determined_username")
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("webauthnAssertion")
	_ = sess.Delete("linkAccount")
	err = sess.Set("uid", user.ID)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}
	err = sess.Set("uname", user.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(user.Language) == 0 {
		lc := middleware.Locale(resp, req)
		opts := &user_service.UpdateOptions{
			Language: optional.Some(lc.Language()),
		}
		if err := user_service.UpdateUser(req.Context(), user, opts); err != nil {
			log.Error(fmt.Sprintf("Error updating user language [user: %d, locale: %s]", user.ID, user.Language))
			return
		}
	}

	middleware.SetLocaleCookie(resp, user.Language, 0)
}
