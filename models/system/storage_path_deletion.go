// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	PathFile = 1 // PathTypeFile represents a file
	PathDir  = 2 // PathTypeDir represents a directory
)

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

func init() {
	db.RegisterModel(new(StoragePathDeletion))
}

func UpdateDeletionFailure(ctx context.Context, deletion *StoragePathDeletion, err error) error {
	deletion.DeleteFailedCount++
	_, updateErr := db.GetEngine(ctx).Table("storage_path_deletion").ID(deletion.ID).Update(map[string]any{
		"delete_failed_count":       deletion.DeleteFailedCount,
		"last_delete_failed_reason": err.Error(),
		"last_delete_failed_time":   timeutil.TimeStampNow(),
	})
	return updateErr
}
