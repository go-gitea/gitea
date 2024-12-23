// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddBlockAdminMergeOverrideBranchProtection(x *xorm.Engine) error {
	type ProtectedBranch struct {
		BlockAdminMergeOverride bool `xorm:"NOT NULL DEFAULT false"`
	}
	return x.Sync(new(ProtectedBranch))
}
