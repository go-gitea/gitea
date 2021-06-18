// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/pam"
	"code.gitea.io/gitea/modules/setting"

	"github.com/google/uuid"
)

// __________  _____      _____
// \______   \/  _  \    /     \
//  |     ___/  /_\  \  /  \ /  \
//  |    |  /    |    \/    Y    \
//  |____|  \____|__  /\____|__  /
//                  \/         \/

// Login queries if login/password is valid against the PAM,
// and create a local user if success when enabled.
func Login(user *models.User, login, password string, sourceID int64, cfg *models.PAMConfig) (*models.User, error) {
	pamLogin, err := pam.Auth(cfg.ServiceName, login, password)
	if err != nil {
		if strings.Contains(err.Error(), "Authentication failure") {
			return nil, models.ErrUserNotExist{Name: login}
		}
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	// Allow PAM sources with `@` in their name, like from Active Directory
	username := pamLogin
	email := pamLogin
	idx := strings.Index(pamLogin, "@")
	if idx > -1 {
		username = pamLogin[:idx]
	}
	if models.ValidateEmail(email) != nil {
		if cfg.EmailDomain != "" {
			email = fmt.Sprintf("%s@%s", username, cfg.EmailDomain)
		} else {
			email = fmt.Sprintf("%s@%s", username, setting.Service.NoReplyAddress)
		}
		if models.ValidateEmail(email) != nil {
			email = uuid.New().String() + "@localhost"
		}
	}

	user = &models.User{
		LowerName:   strings.ToLower(username),
		Name:        username,
		Email:       email,
		Passwd:      password,
		LoginType:   models.LoginPAM,
		LoginSource: sourceID,
		LoginName:   login, // This is what the user typed in
		IsActive:    true,
	}
	return user, models.CreateUser(user)
}
