// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddIndexToActionUserID(x db.EngineMigration) error {
	type Action struct {
		UserID int64 `xorm:"INDEX"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Action))
	return err
}
