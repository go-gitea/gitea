// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

type mirrorWithBackupFields struct {
	ForcePushBackup bool `xorm:"NOT NULL DEFAULT false"`
}

func (mirrorWithBackupFields) TableName() string { return "mirror" }

// AddBackupFieldsToMirror adds mirror backup configuration fields.
func AddBackupFieldsToMirror(_ context.Context, x base.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(mirrorWithBackupFields))
	return err
}
