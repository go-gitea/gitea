// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddDeletionAllowlistToBranchProtection(x db.EngineMigration) error {
	type ProtectedBranch struct {
		CanDelete                bool    `xorm:"NOT NULL DEFAULT false"`
		EnableDeletionAllowlist  bool    `xorm:"NOT NULL DEFAULT false"`
		DeletionAllowlistUserIDs []int64 `xorm:"JSON TEXT"`
		DeletionAllowlistTeamIDs []int64 `xorm:"JSON TEXT"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
