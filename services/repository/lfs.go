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
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// GarbageCollectLFSMetaObjectsOptions provides options for GarbageCollectLFSMetaObjects function
type GarbageCollectLFSMetaObjectsOptions struct {
	RepoID                 int64
	Logger                 log.Logger
	AutoFix                bool
	OlderThan              time.Duration
	LastUpdatedMoreThanAgo time.Duration
}

// GarbageCollectLFSMetaObjects garbage collects LFS objects for all repositories
func GarbageCollectLFSMetaObjects(ctx context.Context, opts GarbageCollectLFSMetaObjectsOptions) error {
	log.Trace("Doing: GarbageCollectLFSMetaObjects")
	defer log.Trace("Finished: GarbageCollectLFSMetaObjects")

	if !setting.LFS.StartServer {
		if opts.Logger != nil {
			opts.Logger.Info("LFS support is disabled")
		}
		return nil
	}

	if opts.RepoID == 0 {
		repo, err := repo_model.GetRepositoryByID(ctx, opts.RepoID)
		if err != nil {
			return err
		}
		return GarbageCollectLFSMetaObjectsForRepo(ctx, repo, opts)
	}

	return db.Iterate(
		ctx,
		builder.And(builder.Gt{"id": 0}),
		func(ctx context.Context, repo *repo_model.Repository) error {
			return GarbageCollectLFSMetaObjectsForRepo(ctx, repo, opts)
		},
	)
}

// GarbageCollectLFSMetaObjectsForRepo garbage collects LFS objects for a specific repository
func GarbageCollectLFSMetaObjectsForRepo(ctx context.Context, repo *repo_model.Repository, opts GarbageCollectLFSMetaObjectsOptions) error {
	if opts.Logger != nil {
		opts.Logger.Info("Checking %-v", repo)
	}
	total, orphaned, collected, deleted := 0, 0, 0, 0
	if opts.Logger != nil {
		defer func() {
			if orphaned == 0 {
				opts.Logger.Info("Found %d total LFSMetaObjects in %-v", total, repo)
			} else if !opts.AutoFix {
				opts.Logger.Info("Found %d/%d orphaned LFSMetaObjects in %-v", orphaned, total, repo)
			} else {
				opts.Logger.Info("Collected %d/%d orphaned/%d total LFSMetaObjects in %-v. %d removed from storage.", collected, orphaned, total, repo, deleted)
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

	var olderThan time.Time
	var updatedLessRecentlyThan time.Time

	if opts.OlderThan > 0 {
		olderThan = time.Now().Add(opts.OlderThan)
	}
	if opts.LastUpdatedMoreThanAgo > 0 {
		updatedLessRecentlyThan = time.Now().Add(opts.LastUpdatedMoreThanAgo)
	}

	return git_model.IterateLFSMetaObjectsForRepo(ctx, repo.ID, func(ctx context.Context, metaObject *git_model.LFSMetaObject, count int64) error {
		total++
		pointerSha := git.ComputeBlobHash([]byte(metaObject.Pointer.StringContent()))

		if gitRepo.IsObjectExist(pointerSha.String()) {
			return git_model.MarkLFSMetaObject(ctx, metaObject.ID)
		}
		orphaned++

		if !opts.AutoFix {
			return nil
		}
		// Non-existent pointer file
		_, err = git_model.RemoveLFSMetaObjectByOidFn(ctx, repo.ID, metaObject.Oid, func(count int64) error {
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
		OlderThan:               olderThan,
		UpdatedLessRecentlyThan: updatedLessRecentlyThan,
	})
}
