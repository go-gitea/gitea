// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
)

func SyncRepoLicenses(ctx context.Context) error {
	log.Trace("Doing: SyncRepoLicenses")

	if err := db.Iterate(
		ctx,
		nil,
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before sync repo licenses for %s", repo.FullName())
			default:
			}
			return repo_module.UpdateRepoLicensesByGitRepo(ctx, repo, nil)
		},
	); err != nil {
		log.Trace("Error: SyncRepoLicenses: %v", err)
		return err
	}

	log.Trace("Finished: SyncReposLicenses")
	return nil
}
