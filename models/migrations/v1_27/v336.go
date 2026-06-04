// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

type mirrorWithBackupFields struct {
	ForcePushBackup bool `xorm:"NOT NULL DEFAULT false"`
}

func (mirrorWithBackupFields) TableName() string {
	return "mirror"
}

// AddBackupFieldsToMirror adds mirror backup configuration fields.
func AddBackupFieldsToMirror(x db.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(mirrorWithBackupFields))
	return err
}
