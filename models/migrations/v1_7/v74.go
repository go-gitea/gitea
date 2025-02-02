// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_7 //nolint

import "xorm.io/xorm"

func AddApprovalWhitelistsToProtectedBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ApprovalsWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
		RequiredApprovals         int64   `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(new(ProtectedBranch))
}
