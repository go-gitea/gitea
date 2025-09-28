// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

func CleanupRepo(ctx context.Context) error {
	log.Trace("Doing: CleanupRepo")

	if err := db.Iterate(
		ctx,
		builder.Eq{"is_empty": false},
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before cleanup repo lock files for %s", repo.FullName())
			default:
			}
			return gitrepo.CleanupRepo(ctx, repo)
		},
	); err != nil {
		log.Trace("Error: CleanupRepo: %v", err)
		return err
	}

	log.Trace("Finished: CleanupRepo")
	return nil
}
