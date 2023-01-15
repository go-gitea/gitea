// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// SyncRepositoryConfig rewrites all repositories' git configurations
// to make sure the reflog settings are up-to-date.
func SyncRepositoryConfig(ctx context.Context) error {
	log.Trace("Doing: SyncRepositoryConfig")

	if err := db.Iterate(
		ctx,
		builder.Gt{"id": 0},
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before sync repository config for %s", repo.FullName())
			default:
			}

			if err := git.CreateConfig(repo.RepoPath()); err != nil {
				return fmt.Errorf("SyncRepositoryConfig: %w", err)
			}
			if repo.HasWiki() {
				if err := git.CreateConfig(repo.WikiPath()); err != nil {
					return fmt.Errorf("SyncRepositoryConfig: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: SyncRepositoryConfig")
	return nil
}
