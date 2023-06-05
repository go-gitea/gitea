// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"io"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// UploadAvatar saves custom avatar for user.
func UploadAvatar(u *user_model.User, data []byte) error {
	avatarData, err := avatar.ProcessAvatarImage(data)
	if err != nil {
		return err
	}

	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	u.UseCustomAvatar = true
	u.Avatar = avatar.HashAvatar(u.ID, data)
	if err = user_model.UpdateUserCols(ctx, u, "use_custom_avatar", "avatar"); err != nil {
		return fmt.Errorf("updateUser: %w", err)
	}

	if err := storage.SaveFrom(storage.Avatars, u.CustomAvatarRelativePath(), func(w io.Writer) error {
		_, err := w.Write(avatarData)
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
