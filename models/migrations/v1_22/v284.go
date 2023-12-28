// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AlterBranchNameCollation(x *xorm.Engine) error {
	if setting.Database.Type.IsMySQL() {
		_, err := x.Exec("ALTER TABLE branch MODIFY COLUMN `name` VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL")
		return err
	} else if setting.Database.Type.IsMSSQL() {
		if _, err := x.Exec("DROP INDEX UQE_branch_s ON branch"); err != nil {
			log.Error("Failed to drop index UQE_branch_s on branch: %v", err) // ignore this error, in case the index has been dropped in previous migration
		}
		if _, err := x.Exec("ALTER TABLE branch ALTER COLUMN [name] nvarchar(255) COLLATE Latin1_General_CS_AS NOT NULL"); err != nil {
			return err
		}
		if _, err := x.Exec("CREATE UNIQUE NONCLUSTERED INDEX UQE_branch_s ON branch (repo_id ASC, [name] ASC)"); err != nil {
			return err
		}
	}
	return nil
}
