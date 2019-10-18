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

	deleteList := make(map[string]struct{})
	sessionOpCount := 0
	for _, user := range users {
		oldAvatar := user.Avatar
		newAvatar := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d-%s", user.ID, user.Avatar))))
		if _, err := os.Stat(filepath.Join(setting.AvatarUploadPath, oldAvatar)); err != nil {
			continue
		}

		fr, err := os.Open(filepath.Join(setting.AvatarUploadPath, oldAvatar))
		if err != nil {
			_ = commitSession(sess)
			return fmt.Errorf("os.Open: %v", err)
		}
		defer fr.Close()

		fw, err := os.Create(filepath.Join(setting.AvatarUploadPath, newAvatar))
		if err != nil {
			_ = commitSession(sess)
			return fmt.Errorf("os.Create: %v", err)
		}
		defer fw.Close()

		if _, err := io.Copy(fw, fr); err != nil {
			_ = commitSession(sess)
			return fmt.Errorf("io.Copy: %v", err)
		}

		user.Avatar = newAvatar
		if _, err := sess.ID(user.ID).Update(&user); err != nil {
			return fmt.Errorf("user table update: %v", err)
		}

		deleteList[filepath.Join(setting.AvatarUploadPath, oldAvatar)] = struct{}{}

		if sessionOpCount++; sessionOpCount >= 100 {
			if err := commitSession(sess); err != nil {
				return err
			}
			sessionOpCount = 0
			if err := sess.Begin(); err != nil {
				return err
			}
		}
	}
	if err := commitSession(sess); err != nil {
		return err
	}
	for file := range deleteList {
		if err := os.Remove(file); err != nil {
			return fmt.Errorf("os.Remove: %v", err)
		}
	}
	return nil
}

func commitSession(sess *xorm.Session) error {
	if err := sess.Commit(); err != nil {
		_ = sess.Rollback()
		return fmt.Errorf("db update: %v", err)
	}
	return nil
}
