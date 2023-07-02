// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func BranchColumnNameCollation(x *xorm.Engine) error {
	if setting.Database.Type.IsMySQL() {
		_, err := x.Exec("ALTER TABLE branch MODIFY COLUMN `name` VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL")
		return err
	} else if setting.Database.Type.IsMSSQL() {
		_, err := x.Exec("ALTER TABLE [branch] ALTER COLUMN [name] nvarchar(255) COLLATE Latin1_General_CS_AS NOT NULL;")
		return err
	}
	return nil
}
