// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddDeletionAllowlistToBranchProtection(_ context.Context, x base.EngineMigration) error {
	type ProtectedBranch struct {
		CanDelete                   bool    `xorm:"NOT NULL DEFAULT false"`
		EnableDeletionAllowlist     bool    `xorm:"NOT NULL DEFAULT false"`
		DeletionAllowlistUserIDs    []int64 `xorm:"JSON TEXT"`
		DeletionAllowlistTeamIDs    []int64 `xorm:"JSON TEXT"`
		DeletionAllowlistDeployKeys bool    `xorm:"NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
