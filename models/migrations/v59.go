// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addProtectedBranchMergeWhitelist(x *xorm.Engine) error {
	type ProtectedBranch struct {
		EnableMergeWhitelist  bool    `xorm:"NOT NULL DEFAULT false"`
		MergeWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		MergeWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
