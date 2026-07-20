// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/xorm/schemas"
)

func AddReleaseIDToReaction(x db.EngineMigration) error {
	type Reaction struct {
		ReleaseID int64 `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	}
	if err := x.Sync(new(Reaction)); err != nil {
		return err
	}

	// Drop index s if it exists, then recreate it.
	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "reaction")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "s" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("reaction", idx)); err != nil {
				return err
			}
			break
		}
	}

	newIndex := schemas.NewIndex("s", schemas.UniqueType)
	newIndex.AddColumn("type", "issue_id", "comment_id", "release_id", "user_id", "original_author_id", "original_author")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("reaction", newIndex)); err != nil {
		return err
	}
	return nil
}
