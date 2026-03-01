// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// UploadAvatar saves custom avatar for user.
func UploadAvatar(ctx context.Context, u *user_model.User, data []byte) (string, error) {
	avatarData, err := avatar.ProcessAvatarImage(data)
	oldAvatarHash := ""
	if err != nil {
		return oldAvatarHash, fmt.Errorf("UploadAvatar: failed to process user avatar image: %w", err)
	}

	result := db.WithTx(ctx, func(ctx context.Context) error {
		if err = user_model.RefreshUserCols(ctx, u, "use_custom_avatar", "avatar"); err != nil {
			return fmt.Errorf("UploadAvatar: failed to refresh user avatar: %w", err)
		}
		u.UseCustomAvatar = true
		oldAvatarHash = u.Avatar
		oldPath := u.CustomAvatarRelativePath()
		newAvatar := avatar.HashAvatar(u.ID, data)
		if oldAvatarHash == newAvatar {
			return nil
		}

		u.Avatar = newAvatar
		if err = user_model.UpdateUserCols(ctx, u, "use_custom_avatar", "avatar"); err != nil {
			return fmt.Errorf("UploadAvatar: failed to update user avatar: %w", err)
		}

		if err := storage.SaveFrom(storage.Avatars, u.CustomAvatarRelativePath(), func(w io.Writer) error {
			_, err := w.Write(avatarData)
			return err
		}); err != nil {
			return fmt.Errorf("UploadAvatar: failed to save user avatar %s: %w", u.CustomAvatarRelativePath(), err)
		}

		if len(oldAvatarHash) > 0 {
			if err := storage.Avatars.Delete(oldPath); err != nil {
				log.Warn("UploadAvatar: Deleting avatar %s: %s", oldPath, err)
			}
		}

		return nil
	})

	return oldAvatarHash, result
}

// DeleteAvatar deletes the user's custom avatar.
func DeleteAvatar(ctx context.Context, u *user_model.User) (string, error) {
	oldAvatarHash := ""
	result := db.WithTx(ctx, func(ctx context.Context) error {
		if err := user_model.RefreshUserCols(ctx, u, "avatar, use_custom_avatar"); err != nil {
			return fmt.Errorf("DeleteAvatar: %w", err)
		}
		aPath := u.CustomAvatarRelativePath()
		log.Trace("DeleteAvatar[%d]: %s", u.ID, aPath)

		u.UseCustomAvatar = false
		oldAvatarHash = u.Avatar
		u.Avatar = ""
		if err := user_model.UpdateUserCols(ctx, u, "avatar, use_custom_avatar"); err != nil {
			return fmt.Errorf("DeleteAvatar: %w", err)
		}

		if len(oldAvatarHash) > 0 {
			if err := storage.Avatars.Delete(aPath); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to remove %s: %w", aPath, err)
				}
				log.Warn("Deleting avatar %s but it doesn't exist", aPath)
			}
		}

		return nil
	})

	return oldAvatarHash, result
}
