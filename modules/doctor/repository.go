// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	repo_service "code.gitea.io/gitea/services/repository"

	"xorm.io/builder"
)

func handleDeleteOrphanedRepos(ctx context.Context, logger log.Logger, autofix bool) error {
	test := &consistencyCheck{
		Name:         "Repos with no existing owner",
		Counter:      countOrphanedRepos,
		Fixer:        deleteOrphanedRepos,
		FixedMessage: "Deleted all content related to orphaned repos",
	}
	return test.Run(ctx, logger, autofix)
}

// countOrphanedRepos count repository where user of owner_id do not exist
func countOrphanedRepos(ctx context.Context) (int64, error) {
	return db.CountOrphanedObjects(ctx, "repository", "user", "repository.owner_id=user.id")
}

// deleteOrphanedRepos delete repository where user of owner_id do not exist
func deleteOrphanedRepos(ctx context.Context) (int64, error) {
	batchSize := db.MaxBatchInsertSize("repository")
	e := db.GetEngine(ctx)
	var deleted int64
	adminUser := &user_model.User{IsAdmin: true}

	for {
		select {
		case <-ctx.Done():
			return deleted, ctx.Err()
		default:
			var ids []int64
			if err := e.Table("`repository`").
				Join("LEFT", "`user`", "repository.owner_id=user.id").
				Where(builder.IsNull{"`user`.id"}).
				Select("`repository`.id").Limit(batchSize).Find(&ids); err != nil {
				return deleted, err
			}

			// if we don't get ids we have deleted them all
			if len(ids) == 0 {
				return deleted, nil
			}

			for _, id := range ids {
				if err := repo_service.DeleteRepositoryDirectly(ctx, adminUser, id, true); err != nil {
					return deleted, err
				}
				deleted++
			}
		}
	}
}

func init() {
	Register(&Check{
		Title:     "Deleted all content related to orphaned repos",
		Name:      "delete-orphaned-repos",
		IsDefault: false,
		Run:       handleDeleteOrphanedRepos,
		Priority:  4,
	})
}
