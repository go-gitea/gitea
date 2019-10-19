// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func useNewNameAvatars(x *xorm.Engine) error {
	d, err := os.Open(setting.AvatarUploadPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing to do if AvatarUploadPath does not exist
			return nil
		}
		return err
	}
	names, err := d.Readdirnames(0)
	if err != nil {
		return err
	}

	type User struct {
		ID              int64 `xorm:"pk autoincr"`
		Avatar          string
		UseCustomAvatar bool
	}

	for _, name := range names {
		userID, err := strconv.ParseInt(name, 10, 64)
		if err != nil {
			log.Warn("ignore avatar %s rename: %v", name, err)
			continue
		}

		var user User
		if has, err := x.ID(userID).Get(&user); err != nil {
			return err
		} else if !has {
			return errors.New("Avatar user is not exist")
		}

		fPath := filepath.Join(setting.AvatarUploadPath, name)
		bs, err := ioutil.ReadFile(fPath)
		if err != nil {
			return err
		}

		user.Avatar = fmt.Sprintf("%x", md5.Sum(bs))
		err = os.Rename(fPath, filepath.Join(setting.AvatarUploadPath, user.Avatar))
		if err != nil {
			return err
		}
		_, err = x.ID(userID).Cols("avatar").Update(&user)
		if err != nil {
			return err
		}
	}
	return nil
}
