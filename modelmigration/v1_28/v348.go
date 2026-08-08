// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func AddReleaseIDToReaction(ctx context.Context, x base.EngineMigration) error {
	// Drop the old 6-column UNIQUE(s) index. xorm Sync cannot reliably update an
	// index when its column set changes.
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "reaction")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "s" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("reaction", idx)); err != nil {
				return err
			}
		}
	}

	// Add the release_id column. No index tag on the partial struct: SyncWithOptions
	// would otherwise reconcile indexes from it and recreate a broken unique index on
	// release_id alone, failing with a unique constraint error when rows already exist.
	type Reaction struct {
		ReleaseID int64 `xorm:"NOT NULL DEFAULT(0)"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Reaction)); err != nil {
		return err
	}

	// Create the new 7-column UNIQUE(s) index; xorm prefixes the name to UQE_reaction_s.
	newIndex := schemas.NewIndex("s", schemas.UniqueType)
	newIndex.AddColumn("type", "issue_id", "comment_id", "release_id", "user_id", "original_author_id", "original_author")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("reaction", newIndex)); err != nil {
		return err
	}

	return nil
}
