// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

// MakePackageVersionUniqueByVersion moves the "s" unique index of package_version from
// (package_id, lower_version) to (package_id, version): versions are case-sensitive strings
// for package types like container (the OCI distribution spec treats "v1.0.0-RC1" and
// "v1.0.0-rc1" as different tags), while "lower_version" keeps its regular index for
// case-insensitive lookups. The new index cannot have duplicates: every (package_id, version)
// pair colliding there would also have collided on the old (package_id, lower_version) index.
func MakePackageVersionUniqueByVersion(ctx context.Context, x base.EngineMigration) error {
	// xorm Sync cannot reliably update an index when its column set changes, drop it first.
	// The regular index on lower_version is a separate index and is not affected.
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "package_version")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "s" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("package_version", idx)); err != nil {
				return err
			}
			break
		}
	}

	// Create the unique index on (package_id, version);
	// xorm prefixes the name to UQE_package_version_s, matching the fresh-install schema.
	newIndex := schemas.NewIndex("s", schemas.UniqueType)
	newIndex.AddColumn("package_id", "version")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("package_version", newIndex)); err != nil {
		return err
	}
	return nil
}
