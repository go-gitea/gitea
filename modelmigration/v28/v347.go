// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddWatchOptions(_ context.Context, x base.EngineMigration) error {
	type Watch struct {
		IncludePullRequests bool `xorm:"NOT NULL DEFAULT true"`
		IncludeIssues       bool `xorm:"NOT NULL DEFAULT true"`
		IncludeReleases     bool `xorm:"NOT NULL DEFAULT true"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Watch))
	return err
}
