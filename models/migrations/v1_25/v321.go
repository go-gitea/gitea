// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25 //nolint

import (
	"xorm.io/xorm"
)

func AddBranchCommitsCount(x *xorm.Engine) error {
	type Branch struct {
		CommitCountID string // the commit id of the commit count
		CommitCount   int64  // the number of commits in this branch
	}
	_, err := x.SyncWithOptions(
		xorm.SyncOptions{
			IgnoreConstrains: true,
			IgnoreIndices:    true,
		},
		new(Branch),
	)
	return err
}
