// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddStoragePathDeletion(x *xorm.Engine) error {
	// StoragePathDeletion represents a file or directory that is pending deletion.
	type StoragePathDeletion struct {
		ID                     int64
		StorageName            string             // storage name defines in storage module
		PathType               int                // 1 for file, 2 for directory
		RelativePath           string             `xorm:"TEXT"`
		DeleteFailedCount      int                `xorm:"DEFAULT 0 NOT NULL"` // Number of times the deletion failed, used to prevent infinite loop
		LastDeleteFailedReason string             `xorm:"TEXT"`               // Last reason the deletion failed, used to prevent infinite loop
		LastDeleteFailedTime   timeutil.TimeStamp // Last time the deletion failed, used to prevent infinite loop
		CreatedUnix            timeutil.TimeStamp `xorm:"INDEX created"`
	}

	return x.Sync(new(StoragePathDeletion))
}
