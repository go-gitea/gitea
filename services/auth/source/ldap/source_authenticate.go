// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	auth_module "code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	source_service "code.gitea.io/gitea/services/auth/source"
	user_service "code.gitea.io/gitea/services/user"
)

// Authenticate queries if login/password is valid against the LDAP directory pool,
// and create a local user if success when enabled.
func (source *Source) Authenticate(ctx context.Context, user *user_model.User, userName, password string) (*user_model.User, error) {
	loginName := userName
	if user != nil {
		loginName = user.LoginName
	}
	sr := source.SearchEntry(loginName, password, source.authSource.Type == auth.DLDAP)
	if sr == nil {
		// User not in LDAP, do nothing
		return nil, user_model.ErrUserNotExist{Name: loginName}
	}
	// Fallback.
	// FIXME: this fallback would cause problems when the "Username" attribute is not set and a user inputs their email.
	// In this case, the email would be used as the username, and will cause the "CreateUser" failure for the first login.
	if sr.Username == "" {
		if strings.Contains(userName, "@") {
			log.Error("No username in search result (Username Attribute is not set properly?), using email as username might cause problems")
		}
		sr.Username = userName
	}
	if sr.Mail == "" {
		sr.Mail = fmt.Sprintf("%s@localhost.local", sr.Username)
	}
	isAttributeSSHPublicKeySet := strings.TrimSpace(source.AttributeSSHPublicKey) != ""

	// Update User admin flag if exist
	if isExist, err := user_model.IsUserExist(ctx, 0, sr.Username); err != nil {
		return nil, err
	} else if isExist {
		if user == nil {
			user, err = user_model.GetUserByName(ctx, sr.Username)
			if err != nil {
				return nil, err
			}
		}
		if user != nil && !user.ProhibitLogin {
			opts := &user_service.UpdateOptions{}
			if source.AdminFilter != "" && user.IsAdmin != sr.IsAdmin {
				// Change existing admin flag only if AdminFilter option is set
				opts.IsAdmin = optional.Some(sr.IsAdmin)
			}
			if !sr.IsAdmin && source.RestrictedFilter != "" && user.IsRestricted != sr.IsRestricted {
				// Change existing restricted flag only if RestrictedFilter option is set
				opts.IsRestricted = optional.Some(sr.IsRestricted)
			}
			if opts.IsAdmin.Has() || opts.IsRestricted.Has() {
				if err := user_service.UpdateUser(ctx, user, opts); err != nil {
					return nil, err
				}
			}
		}
	}

	if user != nil {
		if isAttributeSSHPublicKeySet && asymkey_model.SynchronizePublicKeys(ctx, user, source.authSource, sr.SSHPublicKey) {
			if err := asymkey_service.RewriteAllPublicKeys(ctx); err != nil {
				return user, err
			}
		}
	} else {
		user = &user_model.User{
			LowerName:   strings.ToLower(sr.Username),
			Name:        sr.Username,
			FullName:    composeFullName(sr.Name, sr.Surname, sr.Username),
			Email:       sr.Mail,
			LoginType:   source.authSource.Type,
			LoginSource: source.authSource.ID,
			LoginName:   userName,
			IsAdmin:     sr.IsAdmin,
		}
		overwriteDefault := &user_model.CreateUserOverwriteOptions{
			IsRestricted: optional.Some(sr.IsRestricted),
			IsActive:     optional.Some(true),
		}

		err := user_model.CreateUser(ctx, user, &user_model.Meta{}, overwriteDefault)
		if err != nil {
			return user, err
		}

		if isAttributeSSHPublicKeySet && asymkey_model.AddPublicKeysBySource(ctx, user, source.authSource, sr.SSHPublicKey) {
			if err := asymkey_service.RewriteAllPublicKeys(ctx); err != nil {
				return user, err
			}
		}
		if source.AttributeAvatar != "" {
			if err := user_service.UploadAvatar(ctx, user, sr.Avatar); err != nil {
				return user, err
			}
		}
	}

	if source.GroupsEnabled && (source.GroupTeamMap != "" || source.GroupTeamMapRemoval) {
		groupTeamMapping, err := auth_module.UnmarshalGroupTeamMapping(source.GroupTeamMap)
		if err != nil {
			return user, err
		}
		if err := source_service.SyncGroupsToTeams(ctx, user, sr.Groups, groupTeamMapping, source.GroupTeamMapRemoval); err != nil {
			return user, err
		}
	}

	return user, nil
}

// IsSkipLocalTwoFA returns if this source should skip local 2fa for password authentication
func (source *Source) IsSkipLocalTwoFA() bool {
	return source.SkipLocalTwoFA
}
