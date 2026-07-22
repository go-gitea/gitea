// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddBlockAdminMergeOverrideBranchProtection(x base.EngineMigration) error {
	type ProtectedBranch struct {
		BlockAdminMergeOverride bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
