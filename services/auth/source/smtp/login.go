// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package smtp

import (
	"errors"
	"net/smtp"
	"net/textproto"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/util"
)

//   _________   __________________________
//  /   _____/  /     \__    ___/\______   \
//  \_____  \  /  \ /  \|    |    |     ___/
//  /        \/    Y    \    |    |    |
// /_______  /\____|__  /____|    |____|
//         \/         \/

// Login queries if login/password is valid against the SMTP,
// and create a local user if success when enabled.
func Login(user *models.User, login, password string, sourceID int64, cfg *models.SMTPConfig) (*models.User, error) {
	// Verify allowed domains.
	if len(cfg.AllowedDomains) > 0 {
		idx := strings.Index(login, "@")
		if idx == -1 {
			return nil, models.ErrUserNotExist{Name: login}
		} else if !util.IsStringInSlice(login[idx+1:], strings.Split(cfg.AllowedDomains, ","), true) {
			return nil, models.ErrUserNotExist{Name: login}
		}
	}

	var auth smtp.Auth
	if cfg.Auth == PlainAuthentication {
		auth = smtp.PlainAuth("", login, password, cfg.Host)
	} else if cfg.Auth == LoginAuthentication {
		auth = &loginAuthenticator{login, password}
	} else {
		return nil, errors.New("Unsupported SMTP auth type")
	}

	if err := Authenticate(auth, cfg); err != nil {
		// Check standard error format first,
		// then fallback to worse case.
		tperr, ok := err.(*textproto.Error)
		if (ok && tperr.Code == 535) ||
			strings.Contains(err.Error(), "Username and Password not accepted") {
			return nil, models.ErrUserNotExist{Name: login}
		}
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	username := login
	idx := strings.Index(login, "@")
	if idx > -1 {
		username = login[:idx]
	}

	user = &models.User{
		LowerName:   strings.ToLower(username),
		Name:        strings.ToLower(username),
		Email:       login,
		Passwd:      password,
		LoginType:   models.LoginSMTP,
		LoginSource: sourceID,
		LoginName:   login,
		IsActive:    true,
	}
	return user, models.CreateUser(user)
}
