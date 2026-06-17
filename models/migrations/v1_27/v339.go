// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/xorm/schemas"
)

// AddCreatedUnixToActionUserIsDeletedIndex extends the c_u composite index on
// the action table to include created_unix, enabling efficient ORDER BY on the
// dashboard feed query without a full sort of all matching rows.
func AddCreatedUnixToActionUserIsDeletedIndex(x db.EngineMigration) error {
	// xorm Sync cannot reliably update an index when another index already
	// covers the same columns in a different order (Equal() is order-insensitive).
	// Drop the old c_u index explicitly, then recreate it with the new column set.
	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if idx.Name == "c_u" {
			if _, err := x.Exec(x.Dialect().DropIndexSQL("action", idx)); err != nil {
				return err
			}
			break
		}
	}

	newIndex := schemas.NewIndex("c_u", schemas.IndexType)
	newIndex.AddColumn("user_id", "is_deleted", "created_unix")
	if _, err := x.Exec(x.Dialect().CreateIndexSQL("action", newIndex)); err != nil {
		return err
	}
	return nil
}
