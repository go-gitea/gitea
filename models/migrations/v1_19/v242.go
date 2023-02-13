// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm"
)

// AlterPublicGPGImportKeyContentFieldToMediumText: set GPGImportKey Content field to MEDIUMTEXT
func AlterPublicGPGImportKeyContentFieldToMediumText(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.UseMySQL {
		if _, err := sess.Exec("ALTER TABLE `gpg_import_key` CHANGE `content` `content` MEDIUMTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
