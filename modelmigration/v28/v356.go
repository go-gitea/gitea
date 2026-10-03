// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddPackagePropertyRefIndex indexes package_property by (ref_type, ref_id), as SQLite without ANALYZE may otherwise pick the low-selectivity ref_type index.
func AddPackagePropertyRefIndex(_ context.Context, x base.EngineMigration) error {
	type PackageProperty struct {
		RefType int64 `xorm:"INDEX(ref)"`
		RefID   int64 `xorm:"INDEX(ref)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(PackageProperty))
	return err
}
