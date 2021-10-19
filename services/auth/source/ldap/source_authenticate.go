// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/services/mailer"
)

// Authenticate queries if login/password is valid against the LDAP directory pool,
// and create a local user if success when enabled.
func (source *Source) Authenticate(user *models.User, userName, password string) (*models.User, error) {
	sr := source.SearchEntry(userName, password, source.loginSource.Type == login.DLDAP)
	if sr == nil {
		// User not in LDAP, do nothing
		return nil, models.ErrUserNotExist{Name: userName}
	}

	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0

	// Update User admin flag if exist
	if isExist, err := models.IsUserExist(0, sr.Username); err != nil {
		return nil, err
	} else if isExist {
		if user == nil {
			user, err = models.GetUserByName(sr.Username)
			if err != nil {
				return nil, err
			}
		}
		if user != nil && !user.ProhibitLogin {
			cols := make([]string, 0)
			if len(source.AdminFilter) > 0 && user.IsAdmin != sr.IsAdmin {
				// Change existing admin flag only if AdminFilter option is set
				user.IsAdmin = sr.IsAdmin
				cols = append(cols, "is_admin")
			}
			if !user.IsAdmin && len(source.RestrictedFilter) > 0 && user.IsRestricted != sr.IsRestricted {
				// Change existing restricted flag only if RestrictedFilter option is set
				user.IsRestricted = sr.IsRestricted
				cols = append(cols, "is_restricted")
			}
			if len(cols) > 0 {
				err = models.UpdateUserCols(user, cols...)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if user != nil {
		if isAttributeSSHPublicKeySet && models.SynchronizePublicKeys(user, source.loginSource, sr.SSHPublicKey) {
			return user, models.RewriteAllPublicKeys()
		}

		return user, nil
	}

	// Fallback.
	if len(sr.Username) == 0 {
		sr.Username = userName
	}

	if len(sr.Mail) == 0 {
		sr.Mail = fmt.Sprintf("%s@localhost", sr.Username)
	}

	user = &models.User{
		LowerName:    strings.ToLower(sr.Username),
		Name:         sr.Username,
		FullName:     composeFullName(sr.Name, sr.Surname, sr.Username),
		Email:        sr.Mail,
		LoginType:    source.loginSource.Type,
		LoginSource:  source.loginSource.ID,
		LoginName:    userName,
		IsActive:     true,
		IsAdmin:      sr.IsAdmin,
		IsRestricted: sr.IsRestricted,
	}

	err := models.CreateUser(user)
	if err != nil {
		return user, err
	}

	mailer.SendRegisterNotifyMail(user)

	if isAttributeSSHPublicKeySet && models.AddPublicKeysBySource(user, source.loginSource, sr.SSHPublicKey) {
		err = models.RewriteAllPublicKeys()
	}

	if err == nil && len(source.AttributeAvatar) > 0 {
		_ = user.UploadAvatar(sr.Avatar)
	}

	return user, err
}

// IsSkipLocalTwoFA returns if this source should skip local 2fa for password authentication
func (source *Source) IsSkipLocalTwoFA() bool {
	return source.SkipLocalTwoFA
}
