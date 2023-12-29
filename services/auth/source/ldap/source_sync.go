// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
	auth_module "code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
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
				err = asymkey_model.RewriteAllPublicKeys(ctx)
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
				IsRestricted: util.OptionalBoolOf(su.IsRestricted),
				IsActive:     util.OptionalBoolTrue,
			}

			err = user_model.CreateUser(ctx, usr, overwriteDefault)
			if err != nil {
				log.Error("SyncExternalUsers[%s]: Error creating user %s: %v", source.authSource.Name, su.Username, err)
			} else {
				audit.Record(ctx, audit_model.UserCreate, audit.NewAuthenticationSourceUser(), usr, usr, "Created user %s.", usr.Name)

				if isAttributeSSHPublicKeySet {
					log.Trace("SyncExternalUsers[%s]: Adding LDAP Public SSH Keys for user %s", source.authSource.Name, usr.Name)
					if addedKeys := asymkey_model.AddPublicKeysBySource(ctx, usr, source.authSource, su.SSHPublicKey); len(addedKeys) > 0 {
						sshKeysNeedUpdate = true

						for _, key := range addedKeys {
							audit.Record(ctx, audit_model.UserKeySSHAdd, audit.NewAuthenticationSourceUser(), usr, usr, "Added SSH key %s.", key.Fingerprint)
						}
					}
				}

				if len(source.AttributeAvatar) > 0 {
					_ = user_service.UploadAvatar(ctx, usr, su.Avatar)
				}
			}
		} else if updateExisting {
			// Synchronize SSH Public Key if that attribute is set
			if isAttributeSSHPublicKeySet {
				if addedKeys, deletedKeys := asymkey_model.SynchronizePublicKeys(ctx, usr, source.authSource, su.SSHPublicKey); len(addedKeys) > 0 || len(deletedKeys) > 0 {
					sshKeysNeedUpdate = true

					for _, key := range addedKeys {
						audit.Record(ctx, audit_model.UserKeySSHAdd, audit.NewAuthenticationSourceUser(), usr, usr, "Added SSH key %s.", key.Fingerprint)
					}
					for _, key := range deletedKeys {
						audit.Record(ctx, audit_model.UserKeySSHRemove, audit.NewAuthenticationSourceUser(), usr, usr, "Removed SSH key %s.", key.Fingerprint)
					}
				}
			}

			// Check if user data has changed
			if (len(source.AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
				(len(source.RestrictedFilter) > 0 && usr.IsRestricted != su.IsRestricted) ||
				!strings.EqualFold(usr.Email, su.Mail) ||
				usr.FullName != fullName ||
				!usr.IsActive {

				log.Trace("SyncExternalUsers[%s]: Updating user %s", source.authSource.Name, usr.Name)

				usr.FullName = fullName
				emailChanged := usr.Email != su.Mail
				usr.Email = su.Mail
				// Change existing admin flag only if AdminFilter option is set
				isAdminChanged := false
				if len(source.AdminFilter) > 0 {
					isAdminChanged = usr.IsAdmin != su.IsAdmin
					usr.IsAdmin = su.IsAdmin
				}
				// Change existing restricted flag only if RestrictedFilter option is set
				isRestrictedChanged := false
				if !usr.IsAdmin && len(source.RestrictedFilter) > 0 {
					isRestrictedChanged = usr.IsRestricted != su.IsRestricted
					usr.IsRestricted = su.IsRestricted
				}
				isActiveChanged := !usr.IsActive
				usr.IsActive = true

				emailAddress, err := user_model.UpdateOrSetPrimaryEmail(ctx, usr, emailChanged)
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user primary email %s: %v", source.authSource.Name, usr.Name, err)
				}

				err = user_model.UpdateUser(ctx, usr, "full_name", "email", "is_admin", "is_restricted", "is_active")
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user %s: %v", source.authSource.Name, usr.Name, err)
				}

				if emailChanged {
					audit.Record(ctx, audit_model.UserEmailPrimaryChange, audit.NewAuthenticationSourceUser(), usr, emailAddress, "Changed primary email of user %s to %s.", usr.Name, emailAddress.Email)
				}
				if isActiveChanged {
					audit.Record(ctx, audit_model.UserActive, audit.NewAuthenticationSourceUser(), usr, usr, "Changed activation status of user %s to %s.", usr.Name, audit.UserActiveString(usr.IsActive))
				}
				if isAdminChanged {
					audit.Record(ctx, audit_model.UserAdmin, audit.NewAuthenticationSourceUser(), usr, usr, "Changed admin status of user %s to %s.", usr.Name, audit.UserAdminString(usr.IsAdmin))
				}
				if isRestrictedChanged {
					audit.Record(ctx, audit_model.UserRestricted, audit.NewAuthenticationSourceUser(), usr, usr, "Changed restricted status of user %s to %s.", usr.Name, audit.UserRestrictedString(usr.IsRestricted))
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
		err = asymkey_model.RewriteAllPublicKeys(ctx)
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

			usr.IsActive = false

			if err := user_model.UpdateUserCols(ctx, usr, "is_active"); err != nil {
				log.Error("SyncExternalUsers[%s]: Error deactivating user %s: %v", source.authSource.Name, usr.Name, err)
			} else {
				audit.Record(ctx, audit_model.UserActive, audit.NewAuthenticationSourceUser(), usr, usr, "Changed activation status of user %s to %s.", usr.Name, audit.UserActiveString(usr.IsActive))
			}
		}
	}
	return nil
}
