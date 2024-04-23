// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	auth_module "code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	source_service "code.gitea.io/gitea/services/auth/source"
	user_service "code.gitea.io/gitea/services/user"
)

// Sync causes this ldap source to synchronize its users with the db
func (source *Source) Sync(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers[%s]", source.authSource.Name)

	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
	var sshKeysNeedUpdate bool

	// Find all users with this login type - FIXME: Should this be an iterator?
	users, err := user_model.GetUsersBySource(ctx, source.authSource)
	if err != nil {
		log.Error("SyncExternalUsers: %v", err)
		return err
	}
	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled before update of %s", source.authSource.Name)
		return db.ErrCancelledf("Before update of %s", source.authSource.Name)
	default:
	}

	usernameUsers := make(map[string]*user_model.User, len(users))
	mailUsers := make(map[string]*user_model.User, len(users))
	keepActiveUsers := make(container.Set[int64])

	for _, u := range users {
		usernameUsers[u.LowerName] = u
		mailUsers[strings.ToLower(u.Email)] = u
	}

	sr, err := source.SearchEntries()
	if err != nil {
		log.Error("SyncExternalUsers LDAP source failure [%s], skipped", source.authSource.Name)
		return nil
	}

	if len(sr) == 0 {
		if !source.AllowDeactivateAll {
			log.Error("LDAP search found no entries but did not report an error. Refusing to deactivate all users")
			return nil
		}
		log.Warn("LDAP search found no entries but did not report an error. All users will be deactivated as per settings")
	}

	orgCache := make(map[string]*organization.Organization)
	teamCache := make(map[string]*organization.Team)

	groupTeamMapping, err := auth_module.UnmarshalGroupTeamMapping(source.GroupTeamMap)
	if err != nil {
		return err
	}

	for _, su := range sr {
		select {
		case <-ctx.Done():
			log.Warn("SyncExternalUsers: Cancelled at update of %s before completed update of users", source.authSource.Name)
			// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
			if sshKeysNeedUpdate {
				err = asymkey_service.RewriteAllPublicKeys(ctx)
				if err != nil {
					log.Error("RewriteAllPublicKeys: %v", err)
				}
			}
			return db.ErrCancelledf("During update of %s before completed update of users", source.authSource.Name)
		default:
		}
		if len(su.Username) == 0 && len(su.Mail) == 0 {
			continue
		}

		var usr *user_model.User
		if len(su.Username) > 0 {
			usr = usernameUsers[su.LowerName]
		}
		if usr == nil && len(su.Mail) > 0 {
			usr = mailUsers[strings.ToLower(su.Mail)]
		}

		if usr != nil {
			keepActiveUsers.Add(usr.ID)
		} else if len(su.Username) == 0 {
			// we cannot create the user if su.Username is empty
			continue
		}

		if len(su.Mail) == 0 {
			su.Mail = fmt.Sprintf("%s@localhost.local", su.Username)
		}

		fullName := composeFullName(su.Name, su.Surname, su.Username)
		// If no existing user found, create one
		if usr == nil {
			log.Trace("SyncExternalUsers[%s]: Creating user %s", source.authSource.Name, su.Username)

			usr = &user_model.User{
				LowerName:   su.LowerName,
				Name:        su.Username,
				FullName:    fullName,
				LoginType:   source.authSource.Type,
				LoginSource: source.authSource.ID,
				LoginName:   su.Username,
				Email:       su.Mail,
				IsAdmin:     su.IsAdmin,
			}
			overwriteDefault := &user_model.CreateUserOverwriteOptions{
				IsRestricted: optional.Some(su.IsRestricted),
				IsActive:     optional.Some(true),
			}

			err = user_model.CreateUser(ctx, usr, overwriteDefault)
			if err != nil {
				log.Error("SyncExternalUsers[%s]: Error creating user %s: %v", source.authSource.Name, su.Username, err)
			}

			if err == nil && isAttributeSSHPublicKeySet {
				log.Trace("SyncExternalUsers[%s]: Adding LDAP Public SSH Keys for user %s", source.authSource.Name, usr.Name)
				if asymkey_model.AddPublicKeysBySource(ctx, usr, source.authSource, su.SSHPublicKey) {
					sshKeysNeedUpdate = true
				}
			}

			if err == nil && len(source.AttributeAvatar) > 0 {
				_ = user_service.UploadAvatar(ctx, usr, su.Avatar)
			}
		} else if updateExisting {
			// Synchronize SSH Public Key if that attribute is set
			if isAttributeSSHPublicKeySet && asymkey_model.SynchronizePublicKeys(ctx, usr, source.authSource, su.SSHPublicKey) {
				sshKeysNeedUpdate = true
			}

			// Check if user data has changed
			if (len(source.AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
				(len(source.RestrictedFilter) > 0 && usr.IsRestricted != su.IsRestricted) ||
				!strings.EqualFold(usr.Email, su.Mail) ||
				usr.FullName != fullName ||
				!usr.IsActive {
				log.Trace("SyncExternalUsers[%s]: Updating user %s", source.authSource.Name, usr.Name)

				opts := &user_service.UpdateOptions{
					FullName: optional.Some(fullName),
					IsActive: optional.Some(true),
				}
				if source.AdminFilter != "" {
					opts.IsAdmin = optional.Some(su.IsAdmin)
				}
				// Change existing restricted flag only if RestrictedFilter option is set
				if !su.IsAdmin && source.RestrictedFilter != "" {
					opts.IsRestricted = optional.Some(su.IsRestricted)
				}

				if err := user_service.UpdateUser(ctx, usr, opts); err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user %s: %v", source.authSource.Name, usr.Name, err)
				}

				if err := user_service.ReplacePrimaryEmailAddress(ctx, usr, su.Mail); err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user %s primary email %s: %v", source.authSource.Name, usr.Name, su.Mail, err)
				}
			}

			if usr.IsUploadAvatarChanged(su.Avatar) {
				if err == nil && len(source.AttributeAvatar) > 0 {
					_ = user_service.UploadAvatar(ctx, usr, su.Avatar)
				}
			}
		}
		// Synchronize LDAP groups with organization and team memberships
		if source.GroupsEnabled && (source.GroupTeamMap != "" || source.GroupTeamMapRemoval) {
			if err := source_service.SyncGroupsToTeamsCached(ctx, usr, su.Groups, groupTeamMapping, source.GroupTeamMapRemoval, orgCache, teamCache); err != nil {
				log.Error("SyncGroupsToTeamsCached: %v", err)
			}
		}
	}

	// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
	if sshKeysNeedUpdate {
		err = asymkey_service.RewriteAllPublicKeys(ctx)
		if err != nil {
			log.Error("RewriteAllPublicKeys: %v", err)
		}
	}

	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled during update of %s before delete users", source.authSource.Name)
		return db.ErrCancelledf("During update of %s before delete users", source.authSource.Name)
	default:
	}

	// Deactivate users not present in LDAP
	if updateExisting {
		for _, usr := range users {
			if keepActiveUsers.Contains(usr.ID) {
				continue
			}

			log.Trace("SyncExternalUsers[%s]: Deactivating user %s", source.authSource.Name, usr.Name)

			opts := &user_service.UpdateOptions{
				IsActive: optional.Some(false),
			}
			if err := user_service.UpdateUser(ctx, usr, opts); err != nil {
				log.Error("SyncExternalUsers[%s]: Error deactivating user %s: %v", source.authSource.Name, usr.Name, err)
			}
		}
	}
	return nil
}
