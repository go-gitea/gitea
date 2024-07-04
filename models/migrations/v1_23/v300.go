// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddForcePushBranchProtection(x *xorm.Engine) error {
	type ProtectedBranch struct {
		WhitelistUserIDs             []int64 `json:"whitelist_user_ids" xorm:"JSON TEXT"`
		WhitelistTeamIDs             []int64 `json:"whitelist_team_ids" xorm:"JSON TEXT"`
		EnableForcePushAllowlist     bool
		ForcePushAllowlistUserIDs    []int64 `json:"force_push_allowlist_user_ids" xorm:"JSON TEXT"`
		ForcePushAllowlistTeamIDs    []int64 `json:"force_push_allowlist_team_ids" xorm:"JSON TEXT"`
		ForcePushAllowlistDeployKeys bool    `xorm:"NOT NULL DEFAULT false"`
		MergeWhitelistUserIDs        []int64 `json:"merge_whitelist_user_ids" xorm:"JSON TEXT"`
		MergeWhitelistTeamIDs        []int64 `json:"merge_whitelist_team_ids" xorm:"JSON TEXT"`
		ApprovalsWhitelistUserIDs    []int64 `json:"approvals_whitelist_user_ids" xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs    []int64 `json:"approvals_whitelist_team_ids" xorm:"JSON TEXT"`
	}

	return x.Sync(new(ProtectedBranch))
}
