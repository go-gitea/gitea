// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

func GarbageCollectLFSMetaObjects(ctx context.Context, logger log.Logger, autofix bool) error {
	log.Trace("Doing: GarbageCollectLFSMetaObjects")

	if err := db.Iterate(
		ctx,
		builder.And(builder.Gt{"id": 0}),
		func(ctx context.Context, repo *repo_model.Repository) error {
			return GarbageCollectLFSMetaObjectsForRepo(ctx, repo, logger, autofix)
		},
	); err != nil {
		return err
	}

	log.Trace("Finished: GarbageCollectLFSMetaObjects")
	return nil
}

func GarbageCollectLFSMetaObjectsForRepo(ctx context.Context, repo *repo_model.Repository, logger log.Logger, autofix bool) error {
	if logger != nil {
		logger.Info("Checking %-v", repo)
	}
	total, orphaned, collected, deleted := 0, 0, 0, 0
	if logger != nil {
		defer func() {
			if orphaned == 0 {
				logger.Info("Found %d total LFSMetaObjects in %-v", total, repo)
			} else if !autofix {
				logger.Info("Found %d/%d orphaned LFSMetaObjects in %-v", orphaned, total, repo)
			} else {
				logger.Info("Collected %d/%d orphaned/%d total LFSMetaObjects in %-v. %d removed from storage.", collected, orphaned, total, repo, deleted)
			}
		}()
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		log.Error("Unable to open git repository %-v: %v", repo, err)
		return err
	}
	defer gitRepo.Close()

	store := lfs.NewContentStore()

	return git_model.IterateLFSMetaObjectsForRepo(ctx, repo.ID, func(ctx context.Context, metaObject *git_model.LFSMetaObject, count int64) error {
		total++
		pointerSha := git.ComputeBlobHash([]byte(metaObject.Pointer.StringContent()))

		if gitRepo.IsObjectExist(pointerSha.String()) {
			return nil
		}
		orphaned++

		if !autofix {
			return nil
		}
		// Non-existent pointer file
		_, err = git_model.RemoveLFSMetaObjectByOidFn(repo.ID, metaObject.Oid, func(count int64) error {
			if count > 0 {
				return nil
			}

			if err := store.Delete(metaObject.RelativePath()); err != nil {
				log.Error("Unable to remove lfs metaobject %s from store: %v", metaObject.Oid, err)
			}
			deleted++
			return nil
		})
		if err != nil {
			return fmt.Errorf("unable to remove meta-object %s in %s: %w", metaObject.Oid, repo.FullName(), err)
		}
		collected++

		return nil
	}, &git_model.IterateLFSMetaObjectsForRepoOptions{
		// Only attempt to garbage collect lfs meta objects older than a week as the order of git lfs upload
		// and git object upload is not necessarily guaranteed. It's possible to imagine a situation whereby
		// an LFS object is uploaded but the git branch is not uploaded immediately, or there are some rapid
		// changes in new branches that might lead to lfs objects becoming temporarily unassociated with git
		// objects.
		//
		// It is likely that a week is potentially excessive but it should definitely be enough that any
		// unassociated LFS object is genuinely unassociated.
		OlderThan: time.Now().Add(-24 * 7 * time.Hour),
	})
}
