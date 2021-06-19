// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/auth/source/ldap"
	"code.gitea.io/gitea/services/auth/source/pam"
	"code.gitea.io/gitea/services/auth/source/smtp"
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
			if user.IsPasswordSet() && user.ValidatePassword(password) {

				// Update password hash if server password hash algorithm have changed
				if user.PasswdHashAlgo != setting.PasswordHashAlgo {
					if err = user.SetPassword(password); err != nil {
						return nil, err
					}
					if err = models.UpdateUserCols(user, "passwd", "passwd_hash_algo", "salt"); err != nil {
						return nil, err
					}
				}

				// WARN: DON'T check user.IsActive, that will be checked on reqSign so that
				// user could be hint to resend confirm email.
				if user.ProhibitLogin {
					return nil, models.ErrUserProhibitLogin{
						UID:  user.ID,
						Name: user.Name,
					}
				}

				return user, nil
			}

			return nil, models.ErrUserNotExist{UID: user.ID, Name: user.Name}

		default:
			source, err := models.GetLoginSourceByID(user.LoginSource)
			if err != nil {
				return nil, err
			}

			return ExternalUserLogin(user, user.LoginName, password, source)
		}
	}

	sources, err := models.ActiveLoginSources(-1)
	if err != nil {
		return nil, err
	}

	for _, source := range sources {
		if source.IsOAuth2() || source.IsSSPI() {
			// don't try to authenticate against OAuth2 and SSPI sources here
			continue
		}
		authUser, err := ExternalUserLogin(nil, username, password, source)
		if err == nil {
			return authUser, nil
		}

		log.Warn("Failed to login '%s' via '%s': %v", username, source.Name, err)
	}

	return nil, models.ErrUserNotExist{UID: user.ID, Name: user.Name}
}

// ExternalUserLogin attempts a login using external source types.
func ExternalUserLogin(user *models.User, login, password string, source *models.LoginSource) (*models.User, error) {
	if !source.IsActived {
		return nil, models.ErrLoginSourceNotActived
	}

	var err error
	switch source.Type {
	case models.LoginLDAP, models.LoginDLDAP:
		user, err = ldap.Login(user, login, password, source)
	case models.LoginSMTP:
		user, err = smtp.Login(user, login, password, source.ID, source.Cfg.(*models.SMTPConfig))
	case models.LoginPAM:
		user, err = pam.Login(user, login, password, source.ID, source.Cfg.(*models.PAMConfig))
	default:
		return nil, models.ErrUnsupportedLoginType
	}

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
