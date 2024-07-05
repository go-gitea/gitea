// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddForcePushBranchProtection(x *xorm.Engine) error {
	type ProtectedBranch struct {
		WhitelistUserIDs             []int64 `xorm:"whitelist_user_ids JSON TEXT"`
		WhitelistTeamIDs             []int64 `xorm:"whitelist_team_ids JSON TEXT"`
		EnableForcePushAllowlist     bool
		ForcePushAllowlistUserIDs    []int64 `xorm:"force_push_allowlist_user_ids JSON TEXT"`
		ForcePushAllowlistTeamIDs    []int64 `xorm:"force_push_allowlist_team_ids JSON TEXT"`
		ForcePushAllowlistDeployKeys bool    `xorm:"NOT NULL DEFAULT false"`
		MergeWhitelistUserIDs        []int64 `xorm:"merge_whitelist_user_ids JSON TEXT"`
		MergeWhitelistTeamIDs        []int64 `xorm:"merge_whitelist_team_ids JSON TEXT"`
		ApprovalsWhitelistUserIDs    []int64 `xorm:"approvals_whitelist_user_ids JSON TEXT"`
		ApprovalsWhitelistTeamIDs    []int64 `xorm:"approvals_whitelist_team_ids JSON TEXT"`
	}

	return x.Sync(new(ProtectedBranch))
}
