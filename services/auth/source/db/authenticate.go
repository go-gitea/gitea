// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
)

// Authenticate authenticates the provided user against the DB
func Authenticate(user *user_model.User, login, password string) (*user_model.User, error) {
	if user == nil {
		return nil, user_model.ErrUserNotExist{Name: login}
	}

	if !user.IsPasswordSet() || !user.ValidatePassword(password) {
		return nil, user_model.ErrUserNotExist{UID: user.ID, Name: user.Name}
	}

	// Update password hash if server password hash algorithm have changed
	// Or update the password when the salt length doesn't match the current
	// recommended salt length, this in order to migrate user's salts to a more secure salt.
	if user.PasswdHashAlgo != setting.PasswordHashAlgo || len(user.Salt) != user_model.SaltByteLength*2 {
		if err := user.SetPassword(password); err != nil {
			return nil, err
		}
		if err := user_model.UpdateUserCols(db.DefaultContext, user, "passwd", "passwd_hash_algo", "salt"); err != nil {
			return nil, err
		}
	}

	// WARN: DON'T check user.IsActive, that will be checked on reqSign so that
	// user could be hinted to resend confirm email.
	if user.ProhibitLogin {
		return nil, user_model.ErrUserProhibitLogin{
			UID:  user.ID,
			Name: user.Name,
		}
	}

	// attempting to login as a non-user account
	if user.Type != user_model.UserTypeIndividual {
		return nil, user_model.ErrUserProhibitLogin{
			UID:  user.ID,
			Name: user.Name,
		}
	}

	return user, nil
}
