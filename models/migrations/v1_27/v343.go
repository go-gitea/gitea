// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddGroupColumnsToRepositoryTable(x db.EngineMigration) error {
	type Repository struct {
		LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		GroupID        int64  `xorm:"UNIQUE(s) INDEX DEFAULT 0"`
		OwnerID        int64  `xorm:"UNIQUE(s) INDEX"`
		GroupSortOrder int    `xorm:"INDEX"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: false,
		IgnoreIndices:    false,
	}, new(Repository))
	return err
}
