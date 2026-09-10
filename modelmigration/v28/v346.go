// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func AddLicensePathToRepoLicense(ctx context.Context, x base.EngineMigration) error {
	// Drop the old 2-column UNIQUE(s) index on (repo_id, license) and, on
	// re-runs, the index created under the new name.
	// xorm Sync cannot reliably update an index when its column set changes.
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "repo_license")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "s" || idx.Name == "path" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("repo_license", idx)); err != nil {
				return err
			}
		}
	}

	// Add license_path column. The DEFAULT backfills existing rows: all repos
	// created before this migration used the single LICENSE file convention.
	type RepoLicense struct {
		LicensePath string `xorm:"VARCHAR(255) NOT NULL DEFAULT 'LICENSE'"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(RepoLicense)); err != nil {
		return err
	}

	// Create new 3-column UNIQUE(path) index on (repo_id, license, license_path);
	// xorm prefixes the name to UQE_repo_license_path.
	newIndex := schemas.NewIndex("path", schemas.UniqueType)
	newIndex.AddColumn("repo_id", "license", "license_path")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("repo_license", newIndex)); err != nil {
		return err
	}

	return nil
}
