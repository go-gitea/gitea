// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	user_service "code.gitea.io/gitea/services/user"
)

// Sync causes this ldap source to synchronize its users with the db
func (source *Source) Sync(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers[%s]", source.loginSource.Name)

	var existingUsers []int
	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
	var sshKeysNeedUpdate bool

	// Find all users with this login type - FIXME: Should this be an iterator?
	users, err := user_model.GetUsersBySource(source.loginSource)
	if err != nil {
		log.Error("SyncExternalUsers: %v", err)
		return err
	}
	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled before update of %s", source.loginSource.Name)
		return db.ErrCancelledf("Before update of %s", source.loginSource.Name)
	default:
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].LowerName < users[j].LowerName
	})

	sr, err := source.SearchEntries()
	if err != nil {
		log.Error("SyncExternalUsers LDAP source failure [%s], skipped", source.loginSource.Name)
		return nil
	}

	if len(sr) == 0 {
		if !source.AllowDeactivateAll {
			log.Error("LDAP search found no entries but did not report an error. Refusing to deactivate all users")
			return nil
		}
		log.Warn("LDAP search found no entries but did not report an error. All users will be deactivated as per settings")
	}

	sort.Slice(sr, func(i, j int) bool {
		return sr[i].LowerName < sr[j].LowerName
	})

	userPos := 0

	for _, su := range sr {
		select {
		case <-ctx.Done():
			log.Warn("SyncExternalUsers: Cancelled at update of %s before completed update of users", source.loginSource.Name)
			// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
			if sshKeysNeedUpdate {
				err = models.RewriteAllPublicKeys()
				if err != nil {
					log.Error("RewriteAllPublicKeys: %v", err)
				}
			}
			return db.ErrCancelledf("During update of %s before completed update of users", source.loginSource.Name)
		default:
		}
		if len(su.Username) == 0 {
			continue
		}

		if len(su.Mail) == 0 {
			su.Mail = fmt.Sprintf("%s@localhost", su.Username)
		}

		var usr *user_model.User
		for userPos < len(users) && users[userPos].LowerName < su.LowerName {
			userPos++
		}
		if userPos < len(users) && users[userPos].LowerName == su.LowerName {
			usr = users[userPos]
			existingUsers = append(existingUsers, userPos)
		}

		fullName := composeFullName(su.Name, su.Surname, su.Username)
		// If no existing user found, create one
		if usr == nil {
			log.Trace("SyncExternalUsers[%s]: Creating user %s", source.loginSource.Name, su.Username)

			usr = &user_model.User{
				LowerName:    su.LowerName,
				Name:         su.Username,
				FullName:     fullName,
				LoginType:    source.loginSource.Type,
				LoginSource:  source.loginSource.ID,
				LoginName:    su.Username,
				Email:        su.Mail,
				IsAdmin:      su.IsAdmin,
				IsRestricted: su.IsRestricted,
				IsActive:     true,
			}

			err = user_model.CreateUser(usr)

			if err != nil {
				log.Error("SyncExternalUsers[%s]: Error creating user %s: %v", source.loginSource.Name, su.Username, err)
			}

			if err == nil && isAttributeSSHPublicKeySet {
				log.Trace("SyncExternalUsers[%s]: Adding LDAP Public SSH Keys for user %s", source.loginSource.Name, usr.Name)
				if models.AddPublicKeysBySource(usr, source.loginSource, su.SSHPublicKey) {
					sshKeysNeedUpdate = true
				}
			}

			if err == nil && len(source.AttributeAvatar) > 0 {
				_ = user_service.UploadAvatar(usr, su.Avatar)
			}
		} else if updateExisting {
			// Synchronize SSH Public Key if that attribute is set
			if isAttributeSSHPublicKeySet && models.SynchronizePublicKeys(usr, source.loginSource, su.SSHPublicKey) {
				sshKeysNeedUpdate = true
			}

			// Check if user data has changed
			if (len(source.AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
				(len(source.RestrictedFilter) > 0 && usr.IsRestricted != su.IsRestricted) ||
				!strings.EqualFold(usr.Email, su.Mail) ||
				usr.FullName != fullName ||
				!usr.IsActive {

				log.Trace("SyncExternalUsers[%s]: Updating user %s", source.loginSource.Name, usr.Name)

				usr.FullName = fullName
				usr.Email = su.Mail
				// Change existing admin flag only if AdminFilter option is set
				if len(source.AdminFilter) > 0 {
					usr.IsAdmin = su.IsAdmin
				}
				// Change existing restricted flag only if RestrictedFilter option is set
				if !usr.IsAdmin && len(source.RestrictedFilter) > 0 {
					usr.IsRestricted = su.IsRestricted
				}
				usr.IsActive = true

				err = user_model.UpdateUserCols(db.DefaultContext, usr, "full_name", "email", "is_admin", "is_restricted", "is_active")
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user %s: %v", source.loginSource.Name, usr.Name, err)
				}
			}

			if usr.IsUploadAvatarChanged(su.Avatar) {
				if err == nil && len(source.AttributeAvatar) > 0 {
					_ = user_service.UploadAvatar(usr, su.Avatar)
				}

			}
		}
	}

	// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
	if sshKeysNeedUpdate {
		err = models.RewriteAllPublicKeys()
		if err != nil {
			log.Error("RewriteAllPublicKeys: %v", err)
		}
	}

	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled during update of %s before delete users", source.loginSource.Name)
		return db.ErrCancelledf("During update of %s before delete users", source.loginSource.Name)
	default:
	}

	// Deactivate users not present in LDAP
	if updateExisting {
		existPos := 0
		for i, usr := range users {
			for existPos < len(existingUsers) && i > existingUsers[existPos] {
				existPos++
			}
			if usr.IsActive && (existPos >= len(existingUsers) || i < existingUsers[existPos]) {
				log.Trace("SyncExternalUsers[%s]: Deactivating user %s", source.loginSource.Name, usr.Name)

				usr.IsActive = false
				err = user_model.UpdateUserCols(db.DefaultContext, usr, "is_active")
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error deactivating user %s: %v", source.loginSource.Name, usr.Name, err)
				}
			}
		}
	}
	return nil
}
