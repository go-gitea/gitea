// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"

	"xorm.io/builder"
)

// SyncRepositoryHooks rewrites all repositories' pre-receive, update and post-receive hooks
// to make sure the binary and custom conf path are up-to-date.
func SyncRepositoryHooks(ctx context.Context) error {
	log.Trace("Doing: SyncRepositoryHooks")

	if err := db.Iterate(
		db.DefaultContext,
		new(repo_model.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			repo := bean.(*repo_model.Repository)
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before sync repository hooks for %s", repo.FullName())
			default:
			}

			if err := repo_module.CreateDelegateHooks(repo.RepoPath()); err != nil {
				return fmt.Errorf("SyncRepositoryHook: %v", err)
			}
			if repo.HasWiki() {
				if err := repo_module.CreateDelegateHooks(repo.WikiPath()); err != nil {
					return fmt.Errorf("SyncRepositoryHook: %v", err)
				}
			}
			return nil
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: SyncRepositoryHooks")
	return nil
}
