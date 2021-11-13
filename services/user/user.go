// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"crypto/md5"
	"fmt"
	"image/png"
	"io"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

// UploadAvatar saves custom avatar for user.
// FIXME: split uploads to different subdirs in case we have massive users.
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
	if err = user_model.UpdateUserColsCtx(ctx, u, "use_custom_avatar", "avatar"); err != nil {
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
