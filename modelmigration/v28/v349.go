// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddStoragePathToRepository(_ context.Context, x base.EngineMigration) error {
	// Empty by default: existing repositories keep using the legacy
	// `lower(owner)/lower(name).git` convention on disk.
	type Repository struct {
		StoragePath string `xorm:"VARCHAR(255)"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Repository))
	return err
}
