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
	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted,
// unless the user is younger than USER_DELETE_WITH_COMMENTS_MAX_DAYS.
func DeleteUser(u *user_model.User) error {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation.

	// Check ownership of repository.
	count, err := repo_model.CountRepositories(ctx, repo_model.CountRepositoryOptions{OwnerID: u.ID})
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return models.ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = organization.GetOrganizationCount(ctx, u)
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
		err = fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
		_ = admin_model.CreateNotice(ctx, admin_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
		return err
	}

	if u.Avatar != "" {
		avatarPath := u.CustomAvatarRelativePath()
		if err := storage.Avatars.Delete(avatarPath); err != nil {
			err = fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			_ = admin_model.CreateNotice(ctx, admin_model.NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
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
		return fmt.Errorf("updateUser: %v", err)
	}

	if err := storage.SaveFrom(storage.Avatars, u.CustomAvatarRelativePath(), func(w io.Writer) error {
		if err := png.Encode(w, *m); err != nil {
			log.Error("Encode: %v", err)
		}
		return err
	}); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", u.CustomAvatarRelativePath(), err)
	}

	return committer.Commit()
}

// DeleteAvatar deletes the user's custom avatar.
func DeleteAvatar(u *user_model.User) error {
	aPath := u.CustomAvatarRelativePath()
	log.Trace("DeleteAvatar[%d]: %s", u.ID, aPath)
	if len(u.Avatar) > 0 {
		if err := storage.Avatars.Delete(aPath); err != nil {
			return fmt.Errorf("Failed to remove %s: %v", aPath, err)
		}
	}

	u.UseCustomAvatar = false
	u.Avatar = ""
	if _, err := db.GetEngine(db.DefaultContext).ID(u.ID).Cols("avatar, use_custom_avatar").Update(u); err != nil {
		return fmt.Errorf("UpdateUser: %v", err)
	}
	return nil
}
