// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddBlockOnCodeownerReviews adds block on codeowner reviews branch protection
func AddBlockOnCodeownerReviews(x base.EngineMigration) error {
	type ProtectedBranch struct {
		BlockOnCodeownerReviews bool `xorm:"NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ProtectedBranch))
	return err
}
