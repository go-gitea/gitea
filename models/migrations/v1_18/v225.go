// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_18 // nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AlterPublicGPGKeyContentFieldsToMediumText(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.UseMySQL {
		if _, err := sess.Exec("ALTER TABLE `gpg_key` CHANGE `content` `content` MEDIUMTEXT"); err != nil {
			return err
		}
		if _, err := sess.Exec("ALTER TABLE `public_key` CHANGE `content` `content` MEDIUMTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
