// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddDismissApprovalsOnReRequestColumnToProtectedBranch(x *xorm.Engine) error {
	type ProtectedBranch struct {
		DismissApprovalsOnReRequest bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ProtectedBranch))
	return err
}
