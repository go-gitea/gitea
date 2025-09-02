// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23

import "xorm.io/xorm"

func AddForcePushBranchProtection(x *xorm.Engine) error {
	type ProtectedBranch struct {
		CanForcePush                 bool    `xorm:"NOT NULL DEFAULT false"`
		EnableForcePushAllowlist     bool    `xorm:"NOT NULL DEFAULT false"`
		ForcePushAllowlistUserIDs    []int64 `xorm:"JSON TEXT"`
		ForcePushAllowlistTeamIDs    []int64 `xorm:"JSON TEXT"`
		ForcePushAllowlistDeployKeys bool    `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ProtectedBranch))
	return err
}
