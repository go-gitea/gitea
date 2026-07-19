// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_7

import "gitea.dev/modelmigration/base"

func AddApprovalWhitelistsToProtectedBranches(x base.EngineMigration) error {
	type ProtectedBranch struct {
		ApprovalsWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
		RequiredApprovals         int64   `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(new(ProtectedBranch))
}
