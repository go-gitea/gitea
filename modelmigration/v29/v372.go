// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddIndexIssueDependencyDependencyID indexes the lookup of the issues an issue blocks.
func AddIndexIssueDependencyDependencyID(_ context.Context, x base.EngineMigration) error {
	type IssueDependency struct {
		DependencyID int64 `xorm:"INDEX"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(IssueDependency))
	return err
}
