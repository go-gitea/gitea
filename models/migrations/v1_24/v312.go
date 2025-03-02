// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"xorm.io/xorm"
)

type pullAutoMerge struct {
	DeleteBranchAfterMerge bool
}

// TableName return database table name for xorm
func (pullAutoMerge) TableName() string {
	return "pull_auto_merge"
}

func AddDeleteBranchAfterMergeForAutoMerge(x *xorm.Engine) error {
	return x.Sync(new(pullAutoMerge))
}
