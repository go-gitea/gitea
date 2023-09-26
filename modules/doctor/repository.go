// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	repo_service "code.gitea.io/gitea/services/repository"

	"xorm.io/builder"
)

// CountOrphanedRepos count repository where user of owner_id do not exist
func CountOrphanedRepos(ctx context.Context) (int64, error) {
	return db.CountOrphanedObjects(ctx, "repository", "user", "repository.owner_id=user.id")
}

// DeleteOrphanedRepos delete repository where user of owner_id do not exist
func DeleteOrphanedRepos(ctx context.Context) (int64, error) {
	batchSize := db.MaxBatchInsertSize("repository")
	e := db.GetEngine(ctx)
	var deleted int64
	adminUser := &user_model.User{IsAdmin: true}

	for {
		var ids []int64
		e.Table("`repository`").
			Join("LEFT", "`user`", "repository.owner_id=user.id").
			Where(builder.IsNull{"`user`.id"}).
			Select("`repository`.id").Limit(batchSize).Get(&ids)

		// if we don't get ids we deleted them all
		if len(ids) == 0 {
			return deleted, nil
		}

		for _, id := range ids {
			if err := repo_service.DeleteRepositoryDirectly(ctx, adminUser, 0, id); err != nil {
				return deleted, err
			}
			deleted++
		}
	}
}
