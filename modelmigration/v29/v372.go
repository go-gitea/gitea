// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"

	"xorm.io/builder"
)

func MoveLatestReleaseIDToReleasesUnit(ctx context.Context, x base.EngineMigration) error {
	exists, err := x.Dialect().IsColumnExist(x.DB(), ctx, "repository", "latest_release_id")
	if err != nil || !exists {
		return err
	}

	type Repository struct {
		ID              int64
		LatestReleaseID int64
	}
	return base.EngineTransaction(x, func(sess base.Session) error {
		if err := base.IterateTableByColumn(ctx, x, sess, "id", builder.Neq{"latest_release_id": 0}, func(_ctx context.Context, repo *Repository) error {
			_, err := sess.Table("repo_unit").Where(builder.Eq{"repo_id": repo.ID, "type": 4}).Update(map[string]any{"config": fmt.Sprintf(`{"LatestReleaseID":%d}`, repo.LatestReleaseID)})
			return err
		}); err != nil {
			return err
		}
		return base.DropTableColumns(sess, "repository", "latest_release_id")
	})
}
