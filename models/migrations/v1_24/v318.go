// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"xorm.io/xorm"
)

func AddBranchCommitsCount(x *xorm.Engine) error {
	type Branch struct {
		CommitCount int64 // the number of commits in this branch
	}
	return x.Sync(new(Branch))
}
