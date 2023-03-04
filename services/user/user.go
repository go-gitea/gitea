// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"crypto/md5"
	"fmt"
	"image/png"
	"io"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/packages"
)

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
				return fmt.Errorf("SearchRepositoryByName: %w", err)
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

	ctx, committer, err := db.TxContext()
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

	if err := models.DeleteUser(ctx, u, purge); err != nil {
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

// UploadAvatar saves custom avatar for user.
func UploadAvatar(u *user_model.User, data []byte) error {
	m, err := avatar.Prepare(data)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	u.UseCustomAvatar = true
	// Different users can upload same image as avatar
	// If we prefix it with u.ID, it will be separated
	// Otherwise, if any of the users delete his avatar
	// Other users will lose their avatars too.
	u.Avatar = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d-%x", u.ID, md5.Sum(data)))))
	if err = user_model.UpdateUserCols(ctx, u, "use_custom_avatar", "avatar"); err != nil {
		return fmt.Errorf("updateUser: %w", err)
	}

	if err := storage.SaveFrom(storage.Avatars, u.CustomAvatarRelativePath(), func(w io.Writer) error {
		if err := png.Encode(w, *m); err != nil {
			log.Error("Encode: %v", err)
		}
		return err
	}); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", u.CustomAvatarRelativePath(), err)
	}

	return committer.Commit()
}

// DeleteAvatar deletes the user's custom avatar.
func DeleteAvatar(u *user_model.User) error {
	aPath := u.CustomAvatarRelativePath()
	log.Trace("DeleteAvatar[%d]: %s", u.ID, aPath)
	if len(u.Avatar) > 0 {
		if err := storage.Avatars.Delete(aPath); err != nil {
			return fmt.Errorf("Failed to remove %s: %w", aPath, err)
		}
	}

	u.UseCustomAvatar = false
	u.Avatar = ""
	if _, err := db.GetEngine(db.DefaultContext).ID(u.ID).Cols("avatar, use_custom_avatar").Update(u); err != nil {
		return fmt.Errorf("UpdateUser: %w", err)
	}
	return nil
}
