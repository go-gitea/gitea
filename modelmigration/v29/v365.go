// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddCreatorIDToPackageBlobUpload binds blob upload sessions to the user who
// created them so other authenticated users cannot reuse a leaked UUID.
func AddCreatorIDToPackageBlobUpload(_ context.Context, x base.EngineMigration) error {
	type PackageBlobUpload struct {
		CreatorID int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(PackageBlobUpload))
	return err
}
