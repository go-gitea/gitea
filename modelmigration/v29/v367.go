// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddLatestReleaseIDToRepository(_ctx context.Context, engine base.EngineMigration) error {
	type Repository struct {
		LatestReleaseID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := engine.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Repository))
	return err
}
