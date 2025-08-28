// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import "xorm.io/xorm"

func AddCommitStatusSummary2(x *xorm.Engine) error {
	type CommitStatusSummary struct {
		ID        int64  `xorm:"pk autoincr"`
		TargetURL string `xorm:"TEXT"`
	}
	// there is no migrations because if there is no data on this table, it will fall back to get data
	// from commit status
	return x.Sync(new(CommitStatusSummary))
}
