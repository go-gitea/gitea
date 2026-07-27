// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

type pullAutoMerge struct {
	DeleteBranchAfterMerge bool
}

// TableName return database table name for xorm
func (pullAutoMerge) TableName() string {
	return "pull_auto_merge"
}

func AddDeleteBranchAfterMergeForAutoMerge(x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(pullAutoMerge))
	return err
}
