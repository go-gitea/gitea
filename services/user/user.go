// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/agit"
	"code.gitea.io/gitea/services/packages"
	container_service "code.gitea.io/gitea/services/packages/container"
)

// RenameUser renames a user
func RenameUser(ctx context.Context, u *user_model.User, newUserName string) error {
	// Non-local users are not allowed to change their username.
	if !u.IsOrganization() && !u.IsLocal() {
		return user_model.ErrUserIsNotLocal{
			UID:  u.ID,
			Name: u.Name,
		}
	}

	if newUserName == u.Name {
		return user_model.ErrUsernameNotChanged{
			UID:  u.ID,
			Name: u.Name,
		}
	}

	if err := user_model.IsUsableUsername(newUserName); err != nil {
		return err
	}

	onlyCapitalization := strings.EqualFold(newUserName, u.Name)
	oldUserName := u.Name

	if onlyCapitalization {
		u.Name = newUserName
		if err := user_model.UpdateUserCols(ctx, u, "name"); err != nil {
			u.Name = oldUserName
			return err
		}
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	isExist, err := user_model.IsUserExist(ctx, u.ID, newUserName)
	if err != nil {
		return err
	}
	if isExist {
		return user_model.ErrUserAlreadyExist{
			Name: newUserName,
		}
	}

	if err = repo_model.UpdateRepositoryOwnerName(ctx, oldUserName, newUserName); err != nil {
		return err
	}

	if err = user_model.NewUserRedirect(ctx, u.ID, oldUserName, newUserName); err != nil {
		return err
	}

	if err := agit.UserNameChanged(ctx, u, newUserName); err != nil {
		return err
	}
	if err := container_service.UpdateRepositoryNames(ctx, u, newUserName); err != nil {
		return err
	}

	u.Name = newUserName
	u.LowerName = strings.ToLower(newUserName)
	if err := user_model.UpdateUserCols(ctx, u, "name", "lower_name"); err != nil {
		u.Name = oldUserName
		u.LowerName = strings.ToLower(oldUserName)
		return err
	}

	// Do not fail if directory does not exist
	if err = util.Rename(user_model.UserPath(oldUserName), user_model.UserPath(newUserName)); err != nil && !os.IsNotExist(err) {
		u.Name = oldUserName
		u.LowerName = strings.ToLower(oldUserName)
		return fmt.Errorf("rename user directory: %w", err)
	}

	if err = committer.Commit(); err != nil {
		u.Name = oldUserName
		u.LowerName = strings.ToLower(oldUserName)
		if err2 := util.Rename(user_model.UserPath(newUserName), user_model.UserPath(oldUserName)); err2 != nil && !os.IsNotExist(err2) {
			log.Critical("Unable to rollback directory change during failed username change from: %s to: %s. DB Error: %v. Filesystem Error: %v", oldUserName, newUserName, err, err2)
			return fmt.Errorf("failed to rollback directory change during failed username change from: %s to: %s. DB Error: %w. Filesystem Error: %v", oldUserName, newUserName, err, err2)
		}
		return err
	}
	return nil
}

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted,
// unless the user is younger than USER_DELETE_WITH_COMMENTS_MAX_DAYS.
func DeleteUser(ctx context.Context, u *user_model.User, purge bool) error {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}

	if purge {
		// Disable the user first
		// NOTE: This is deliberately not within a transaction as it must disable the user immediately to prevent any further action by the user to be purged.
		if err := user_model.UpdateUserCols(ctx, &user_model.User{
			ID:              u.ID,
			IsActive:        false,
			IsRestricted:    true,
			IsAdmin:         false,
			ProhibitLogin:   true,
			Passwd:          "",
			Salt:            "",
			PasswdHashAlgo:  "",
			MaxRepoCreation: 0,
		}, "is_active", "is_restricted", "is_admin", "prohibit_login", "max_repo_creation", "passwd", "salt", "passwd_hash_algo"); err != nil {
			return fmt.Errorf("unable to disable user: %s[%d] prior to purge. UpdateUserCols: %w", u.Name, u.ID, err)
		}

		// Force any logged in sessions to log out
		// FIXME: We also need to tell the session manager to log them out too.
		eventsource.GetManager().SendMessage(u.ID, &eventsource.Event{
			Name: "logout",
		})

		// Delete all repos belonging to this user
		// Now this is not within a transaction because there are internal transactions within the DeleteRepository
		// BUT: the db will still be consistent even if a number of repos have already been deleted.
		// And in fact we want to capture any repositories that are being created in other transactions in the meantime
		//
		// An alternative option here would be write a DeleteAllRepositoriesForUserID function which would delete all of the repos
		// but such a function would likely get out of date
		for {
			repos, _, err := repo_model.GetUserRepositories(&repo_model.SearchRepoOptions{
				ListOptions: db.ListOptions{
					PageSize: repo_model.RepositoryListDefaultPageSize,
					Page:     1,
				},
				Private: true,
				OwnerID: u.ID,
				Actor:   u,
			})
			if err != nil {
				return fmt.Errorf("GetUserRepositories: %w", err)
			}
			if len(repos) == 0 {
				break
			}
			for _, repo := range repos {
				if err := models.DeleteRepository(u, u.ID, repo.ID); err != nil {
					return fmt.Errorf("unable to delete repository %s for %s[%d]. Error: %w", repo.Name, u.Name, u.ID, err)
				}
			}
		}

		// Remove from Organizations and delete last owner organizations
		// Now this is not within a transaction because there are internal transactions within the DeleteOrganization
		// BUT: the db will still be consistent even if a number of organizations memberships and organizations have already been deleted
		// And in fact we want to capture any organization additions that are being created in other transactions in the meantime
		//
		// An alternative option here would be write a function which would delete all organizations but it seems
		// but such a function would likely get out of date
		for {
			orgs, err := organization.FindOrgs(organization.FindOrgOptions{
				ListOptions: db.ListOptions{
					PageSize: repo_model.RepositoryListDefaultPageSize,
					Page:     1,
				},
				UserID:         u.ID,
				IncludePrivate: true,
			})
			if err != nil {
				return fmt.Errorf("unable to find org list for %s[%d]. Error: %w", u.Name, u.ID, err)
			}
			if len(orgs) == 0 {
				break
			}
			for _, org := range orgs {
				if err := models.RemoveOrgUser(org.ID, u.ID); err != nil {
					if organization.IsErrLastOrgOwner(err) {
						err = organization.DeleteOrganization(ctx, org)
					}
					if err != nil {
						return fmt.Errorf("unable to remove user %s[%d] from org %s[%d]. Error: %w", u.Name, u.ID, org.Name, org.ID, err)
					}
				}
			}
		}

		// Delete Packages
		if setting.Packages.Enabled {
			if _, err := packages.RemoveAllPackages(ctx, u.ID); err != nil {
				return err
			}
		}
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation. This causes a race with the purge above
	//  however consistency requires that we ensure that this is the case

	// Check ownership of repository.
	count, err := repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{OwnerID: u.ID})
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %w", err)
	} else if count > 0 {
		return models.ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = organization.GetOrganizationCount(ctx, u)
	if err != nil {
		return fmt.Errorf("GetOrganizationCount: %w", err)
	} else if count > 0 {
		return models.ErrUserHasOrgs{UID: u.ID}
	}

	// Check ownership of packages.
	if ownsPackages, err := packages_model.HasOwnerPackages(ctx, u.ID); err != nil {
		return fmt.Errorf("HasOwnerPackages: %w", err)
	} else if ownsPackages {
		return models.ErrUserOwnPackages{UID: u.ID}
	}

	if err := deleteUser(ctx, u, purge); err != nil {
		return fmt.Errorf("DeleteUser: %w", err)
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	if err = asymkey_model.RewriteAllPublicKeys(); err != nil {
		return err
	}
	if err = asymkey_model.RewriteAllPrincipalKeys(db.DefaultContext); err != nil {
		return err
	}

	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := user_model.UserPath(u.Name)
	if err := util.RemoveAll(path); err != nil {
		err = fmt.Errorf("Failed to RemoveAll %s: %w", path, err)
		_ = system_model.CreateNotice(ctx, system_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
		return err
	}

	if u.Avatar != "" {
		avatarPath := u.CustomAvatarRelativePath()
		if err := storage.Avatars.Delete(avatarPath); err != nil {
			err = fmt.Errorf("Failed to remove %s: %w", avatarPath, err)
			_ = system_model.CreateNotice(ctx, system_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
			return err
		}
	}

	return nil
}

// DeleteInactiveUsers deletes all inactive users and email addresses.
func DeleteInactiveUsers(ctx context.Context, olderThan time.Duration) error {
	users, err := user_model.GetInactiveUsers(ctx, olderThan)
	if err != nil {
		return err
	}

	// FIXME: should only update authorized_keys file once after all deletions.
	for _, u := range users {
		select {
		case <-ctx.Done():
			return db.ErrCancelledf("Before delete inactive user %s", u.Name)
		default:
		}
		if err := DeleteUser(ctx, u, false); err != nil {
			// Ignore users that were set inactive by admin.
			if models.IsErrUserOwnRepos(err) || models.IsErrUserHasOrgs(err) || models.IsErrUserOwnPackages(err) {
				continue
			}
			return err
		}
	}

	return user_model.DeleteInactiveEmailAddresses(ctx)
}
