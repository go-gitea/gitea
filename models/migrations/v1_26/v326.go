// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddBranchProtectionBypassAllowlist(x *xorm.Engine) error {
	type ProtectedBranch struct {
		EnableBypassAllowlist  bool    `xorm:"NOT NULL DEFAULT false"`
		BypassAllowlistUserIDs []int64 `xorm:"JSON TEXT"`
		BypassAllowlistTeamIDs []int64 `xorm:"JSON TEXT"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
