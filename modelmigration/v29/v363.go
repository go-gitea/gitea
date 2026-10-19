// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func AddTeamIDToPackage(ctx context.Context, x base.EngineMigration) error {
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "package")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "s" || idx.Name == "UQE_package_s" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("package", idx)); err != nil {
				return err
			}
		}
	}

	type Package struct {
		TeamID int64 `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Package)); err != nil {
		return err
	}

	newIndex := schemas.NewIndex("s", schemas.UniqueType)
	newIndex.AddColumn("owner_id", "team_id", "type", "lower_name")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("package", newIndex)); err != nil {
		return err
	}

	return nil
}
