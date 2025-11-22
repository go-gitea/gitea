// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddGroupColumnsToRepositoryTable(x *xorm.Engine) error {
	type Repository struct {
		GroupID        int64 `xorm:"UNIQUE(s) INDEX DEFAULT NULL"`
		GroupSortOrder int
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: false,
		IgnoreIndices:    false,
	}, new(Repository))
	return err
}
