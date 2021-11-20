// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	admin_model "code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted,
// unless the user is younger than USER_DELETE_WITH_COMMENTS_MAX_DAYS.
func DeleteUser(u *models.User) error {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}

	ctx, commiter, err := db.TxContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation.

	// Check ownership of repository.
	count, err := models.GetRepositoryCount(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return models.ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = models.GetOrganizationCount(ctx, u)
	if err != nil {
		return fmt.Errorf("GetOrganizationCount: %v", err)
	} else if count > 0 {
		return models.ErrUserHasOrgs{UID: u.ID}
	}

	// Check ownership of packages.
	if ownsPackages, err := packages_model.HasOwnerPackages(ctx, u.ID); err != nil {
		return fmt.Errorf("HasOwnerPackages: %v", err)
	} else if ownsPackages {
		return models.ErrUserOwnPackages{UID: u.ID}
	}

	if err := models.DeleteUser(ctx, u); err != nil {
		return fmt.Errorf("DeleteUser: %v", err)
	}

	if err := commiter.Commit(); err != nil {
		return err
	}

	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := models.UserPath(u.Name)
	if err := util.RemoveAll(path); err != nil {
		err = fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
		_ = admin_model.CreateNotice(db.DefaultContext, admin_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
		return err
	}

	if u.Avatar != "" {
		avatarPath := u.CustomAvatarRelativePath()
		if err := storage.Avatars.Delete(avatarPath); err != nil {
			err = fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			_ = admin_model.CreateNotice(db.DefaultContext, admin_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
			return err
		}
	}

	return nil
}

// DeleteInactiveUsers deletes all inactive users and email addresses.
func DeleteInactiveUsers(ctx context.Context, olderThan time.Duration) error {
	users, err := models.GetInactiveUsers(ctx, olderThan)
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
		if err := DeleteUser(u); err != nil {
			// Ignore users that were set inactive by admin.
			if models.IsErrUserOwnRepos(err) || models.IsErrUserHasOrgs(err) || models.IsErrUserOwnPackages(err) {
				continue
			}
			return err
		}
	}

	return user_model.DeleteInactiveEmailAddresses(ctx)
}
