// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

func AddBlockAdminMergeOverrideBranchProtection(x db.EngineMigration) error {
	type ProtectedBranch struct {
		BlockAdminMergeOverride bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
