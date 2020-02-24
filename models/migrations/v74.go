// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addApprovalWhitelistsToProtectedBranches(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ApprovalsWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
		RequiredApprovals         int64   `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync2(new(ProtectedBranch))
}
