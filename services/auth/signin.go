// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/auth/source/db"

	// Register the other sources
	_ "code.gitea.io/gitea/services/auth/source/ldap"
	_ "code.gitea.io/gitea/services/auth/source/oauth2"
	_ "code.gitea.io/gitea/services/auth/source/pam"
	_ "code.gitea.io/gitea/services/auth/source/smtp"
	_ "code.gitea.io/gitea/services/auth/source/sspi"
)

// UserSignIn validates user name and password.
func UserSignIn(username, password string) (*models.User, error) {
	var user *models.User
	if strings.Contains(username, "@") {
		user = &models.User{Email: strings.ToLower(strings.TrimSpace(username))}
		// check same email
		cnt, err := models.Count(user)
		if err != nil {
			return nil, err
		}
		if cnt > 1 {
			return nil, models.ErrEmailAlreadyUsed{
				Email: user.Email,
			}
		}
	} else {
		trimmedUsername := strings.TrimSpace(username)
		if len(trimmedUsername) == 0 {
			return nil, models.ErrUserNotExist{Name: username}
		}

		user = &models.User{LowerName: strings.ToLower(trimmedUsername)}
	}

	hasUser, err := models.GetUser(user)
	if err != nil {
		return nil, err
	}

	if hasUser {
		switch user.LoginType {
		case models.LoginNoType, models.LoginPlain, models.LoginOAuth2:
			return db.Authenticate(user, user.Name, password)
		default:
			source, err := models.GetLoginSourceByID(user.LoginSource)
			if err != nil {
				return nil, err
			}

			if !source.IsActived {
				return nil, models.ErrLoginSourceNotActived
			}

			authenticator, ok := source.Cfg.(PasswordAuthenticator)
			if !ok {
				return nil, models.ErrUnsupportedLoginType

			}

			user, err := authenticator.Authenticate(nil, username, password)
			if err != nil {
				return nil, err
			}

			// WARN: DON'T check user.IsActive, that will be checked on reqSign so that
			// user could be hint to resend confirm email.
			if user.ProhibitLogin {
				return nil, models.ErrUserProhibitLogin{UID: user.ID, Name: user.Name}
			}

			return user, nil
		}
	}

	sources, err := models.ActiveLoginSources(-1)
	if err != nil {
		return nil, err
	}

	for _, source := range sources {
		if !source.IsActived {
			// don't try to authenticate non-active sources
			continue
		}

		authenticator, ok := source.Cfg.(PasswordAuthenticator)
		if !ok {
			continue
		}

		authUser, err := authenticator.Authenticate(nil, username, password)

		if err == nil {
			if !user.ProhibitLogin {
				return authUser, nil
			}
			err = models.ErrUserProhibitLogin{UID: user.ID, Name: user.Name}
		}

		log.Warn("Failed to login '%s' via '%s': %v", username, source.Name, err)
	}

	return nil, models.ErrUserNotExist{UID: user.ID, Name: user.Name}
}
