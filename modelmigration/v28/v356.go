// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddPackagePropertyRefIndex indexes package_property by (ref_type, ref_id).
// Property lookups always filter on both columns. With only single-column indexes,
// SQLite (which has no ANALYZE statistics by default) can pick the low-selectivity
// ref_type index and scan every property of that type for each lookup.
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
