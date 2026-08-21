// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddPublishedUnixToRelease(_ context.Context, x base.EngineMigration) error {
	type Release struct {
		PublishedUnix int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Release)); err != nil {
		return err
	}

	// existing rows have no recorded publication time, so fall back to their creation time
	_, err := x.Exec("UPDATE `release` SET published_unix = created_unix WHERE published_unix = 0 AND is_draft = ?", false)
	return err
}
