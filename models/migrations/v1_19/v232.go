// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 // nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AlterPackageVersionMetadataToLongText(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if setting.Database.UseMySQL {
		if _, err := sess.Exec("ALTER TABLE `package_version` MODIFY COLUMN `metadata_json` LONGTEXT"); err != nil {
			return err
		}
	}
	return sess.Commit()
}
