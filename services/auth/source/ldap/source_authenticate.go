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
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
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
	if len(sr.Username) == 0 {
		sr.Username = userName
	}
	if len(sr.Mail) == 0 {
		sr.Mail = fmt.Sprintf("%s@localhost.local", sr.Username)
	}
	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0

	// Update User admin flag if exist
	isAdminChanged := false
	isRestrictedChanged := false
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
			cols := make([]string, 0)
			if len(source.AdminFilter) > 0 && user.IsAdmin != sr.IsAdmin {
				// Change existing admin flag only if AdminFilter option is set
				user.IsAdmin = sr.IsAdmin
				cols = append(cols, "is_admin")

				isAdminChanged = true
			}
			if !user.IsAdmin && len(source.RestrictedFilter) > 0 && user.IsRestricted != sr.IsRestricted {
				// Change existing restricted flag only if RestrictedFilter option is set
				user.IsRestricted = sr.IsRestricted
				cols = append(cols, "is_restricted")

				isRestrictedChanged = true
			}
			if len(cols) > 0 {
				err = user_model.UpdateUserCols(ctx, user, cols...)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if user != nil {
		if isAdminChanged {
			audit.Record(audit.UserAdmin, audit.NewAuthenticationSourceUser(), user, user, "Admin status of user %s changed to %s.", user.Name, audit.UserAdminString(user.IsAdmin))
		}
		if isRestrictedChanged {
			audit.Record(audit.UserRestricted, audit.NewAuthenticationSourceUser(), user, user, "Restricted status of user %s changed to %s.", user.Name, audit.UserRestrictedString(user.IsRestricted))
		}

		if isAttributeSSHPublicKeySet {
			if addedKeys, deletedKeys := asymkey_model.SynchronizePublicKeys(ctx, user, source.authSource, sr.SSHPublicKey); len(addedKeys) > 0 || len(deletedKeys) > 0 {
				for _, key := range addedKeys {
					audit.Record(audit.UserKeySSHAdd, audit.NewAuthenticationSourceUser(), user, user, "Added SSH key %s.", key.Fingerprint)
				}
				for _, key := range deletedKeys {
					audit.Record(audit.UserKeySSHRemove, audit.NewAuthenticationSourceUser(), user, user, "Removed SSH key %s.", key.Fingerprint)
				}

				if err := asymkey_model.RewriteAllPublicKeys(ctx); err != nil {
					return user, err
				}
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
			IsRestricted: util.OptionalBoolOf(sr.IsRestricted),
			IsActive:     util.OptionalBoolTrue,
		}

		err := user_model.CreateUser(ctx, user, overwriteDefault)
		if err != nil {
			return user, err
		}

		audit.Record(audit.UserCreate, audit.NewAuthenticationSourceUser(), user, user, "Created user %s.", user.Name)

		if isAttributeSSHPublicKeySet {
			if addedKeys := asymkey_model.AddPublicKeysBySource(ctx, user, source.authSource, sr.SSHPublicKey); len(addedKeys) > 0 {
				for _, key := range addedKeys {
					audit.Record(audit.UserKeySSHAdd, audit.NewAuthenticationSourceUser(), user, user, "Added SSH key %s.", key.Fingerprint)
				}

				if err := asymkey_model.RewriteAllPublicKeys(ctx); err != nil {
					return user, err
				}
			}
		}
		if len(source.AttributeAvatar) > 0 {
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
