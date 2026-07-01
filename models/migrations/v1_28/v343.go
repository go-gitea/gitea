// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddCreatorIDToPackageBlobUpload binds blob upload sessions to the user who
// created them so other authenticated users cannot reuse a leaked UUID.
func AddCreatorIDToPackageBlobUpload(x db.EngineMigration) error {
	type PackageBlobUpload struct {
		CreatorID int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(PackageBlobUpload))
	return err
}
