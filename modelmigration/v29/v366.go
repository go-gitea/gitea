// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddMetadataSyncOptionsToMirror(_ context.Context, x base.EngineMigration) error {
	type Mirror struct {
		SyncIssues       bool `xorm:"NOT NULL DEFAULT false"`
		SyncPullRequests bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Mirror))
	return err
}
