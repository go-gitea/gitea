// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AddOwnerIDToProtectedBranch(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ID      int64 `xorm:"pk autoincr"`
		RepoID  int64 `xorm:"INDEX DEFAULT 0"`
		OwnerID int64 `xorm:"INDEX DEFAULT 0"`
	}

	if err := x.Sync(new(ProtectedBranch)); err != nil {
		return err
	}

	db := x.NewSession()
	defer db.Close()

	if err := db.Begin(); err != nil {
		return err
	}

	// Drop old unique index if it exists, ignoring errors if it doesn't.
	if _, err := db.Exec("DROP INDEX `UQE_protected_branch_s`"); err != nil {
		return err
		//log.Warn("Could not drop index UQE_protected_branch_s: %v", err)
	}

	// These partial indexes might not be supported on all database versions, but they are the correct approach.
	// We will assume modern database versions. The WHERE clause might need adjustment for MSSQL.
	if !setting.Database.Type.IsMSSQL() {
		if _, err := db.Exec("CREATE UNIQUE INDEX `UQE_protected_branch_repo_id_branch_name` ON `protected_branch` (`repo_id`, `branch_name`) WHERE `repo_id` != 0"); err != nil {
			return err
		}
		if _, err := db.Exec("CREATE UNIQUE INDEX `UQE_protected_branch_owner_id_branch_name` ON `protected_branch` (`owner_id`, `branch_name`) WHERE `owner_id` != 0"); err != nil {
			return err
		}
	}

	return db.Commit()
}
