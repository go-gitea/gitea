// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"context"
	"os"
	"path/filepath"

	"gitea.dev/models/db"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
)

// RecoverOffline requires every writer sharing this database and storage to have stopped.
func RecoverOffline(ctx context.Context) error {
	var operations []*quota_model.Operation
	if err := db.GetEngine(ctx).Find(&operations); err != nil {
		return err
	}
	for _, op := range operations {
		paths, err := filepath.Glob(filepath.Join(setting.RepoRootPath, "*", ".gitea-quota-"+op.Token+"-*"))
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err = os.RemoveAll(path); err != nil {
				return err
			}
		}
		var repos []*repo_model.Repository
		if err = db.GetEngine(ctx).Where("owner_id=?", op.OwnerID).Find(&repos); err != nil {
			return err
		}
		for _, repo := range repos {
			if err = cleanupIncoming(repo); err != nil {
				return err
			}
			if repo.ID == op.RepoID {
				for _, target := range []git.RepositoryFacade{repo.CodeStorageRepo(), repo.WikiStorageRepo()} {
					exists, err := git.IsRepositoryExist(ctx, target)
					if err != nil {
						return err
					}
					if exists {
						if err = gitcmd.NewCommand("fsck", "--full", "--no-dangling").WithRepo(target).Run(ctx); err != nil {
							return err
						}
					}
				}
			}
			if _, err = Reconcile(ctx, repo); err != nil {
				return err
			}
		}
		if err = quota_model.ResetChildrenOffline(ctx, op); err != nil {
			return err
		}
		if err = quota_model.Release(ctx, op); err != nil {
			return err
		}
	}
	return nil
}

func ReconcileAll(ctx context.Context) error {
	enabled, err := Enabled(ctx)
	if err != nil || !enabled {
		return err
	}
	return db.Iterate(ctx, nil, func(ctx context.Context, repo *repo_model.Repository) error {
		_, op, err := Begin(ctx, repo)
		if err != nil {
			if IsQuotaError(err) {
				return nil
			}
			return err
		}
		op.Finish()
		return nil
	})
}
