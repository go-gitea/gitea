// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func renameExistingUserAvatarName(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	type User struct {
		ID     int64 `xorm:"pk autoincr"`
		Avatar string
	}

	users := make([]User, 0)
	if err := sess.Find(&users); err != nil {
		return err
	}

	deleteList := make([]string, 0, len(users))
	for _, user := range users {
		oldAvatar := user.Avatar
		newAvatar := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d-%s", user.ID, user.Avatar))))

		if _, err := os.Stat(filepath.Join(setting.AvatarUploadPath, oldAvatar)); err != nil {
			continue
		}

		fr, err := os.Open(filepath.Join(setting.AvatarUploadPath, oldAvatar))
		if err != nil {
			return err
		}
		defer fr.Close()

		fw, err := os.Create(filepath.Join(setting.AvatarUploadPath, newAvatar))
		if err != nil {
			return err
		}
		defer fw.Close()

		if _, err := io.Copy(fw, fr); err != nil {
			return err
		}

		user.Avatar = newAvatar
		if _, err := sess.ID(user.ID).Update(&user); err != nil {
			return err
		}

		deleteList = append(deleteList, filepath.Join(setting.AvatarUploadPath, oldAvatar))
	}
	for _, file := range deleteList {
		if err := os.Remove(file); err != nil {
			return err
		}
	}
	return sess.Commit()
}
