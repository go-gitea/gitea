// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"os"
	"path"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	Register(&Check{
		Title:     "Migrate LFS objects to new path format",
		Name:      "lfs-migrate-object-paths",
		IsDefault: false,
		Run:       runLFSMigrateObjectPaths,
		Priority:  8, // Run after other LFS checks
	})
}

func runLFSMigrateObjectPaths(ctx context.Context, logger log.Logger, autofix bool) error {
	if !setting.LFS.StartServer {
		logger.Info("LFS support is disabled, skipping.")
		return nil
	}

	if autofix {
		logger.Info("Migrating LFS objects...")
	} else {
		logger.Info("Checking for LFS objects that need migration (dry run)...")
	}

	return lfsMigratePaths(ctx, logger, autofix)
}

func lfsMigratePaths(ctx context.Context, logger log.Logger, fix bool) error {
	var migratedCount int64

	oldPath := func(oid string) string {
		return path.Join(setting.LFS.Storage.Path, oid[0:2], oid[2:4], oid[4:])
	}
	newPath := func(oid string) string {
		return path.Join(setting.LFS.Storage.Path, oid[0:2], oid[2:4], oid)
	}

	err := git_model.IterateRepositoryIDsWithLFSMetaObjects(ctx, func(ctx context.Context, repoID, count int64) error {
		return git_model.IterateLFSMetaObjectsForRepo(ctx, repoID, func(ctx context.Context, meta *git_model.LFSMetaObject, count int64) error {
			oldOidPath := oldPath(meta.Oid)
			_, err := os.Stat(oldOidPath)
			if err != nil {
				if os.IsNotExist(err) {
					return nil // Does not exist at old path, so nothing to do.
				}
				logger.Error("Error checking for LFS object at %s: %v", oldOidPath, err)
				return err
			}

			migratedCount++
			newOidPath := newPath(meta.Oid)
			if fix {
				logger.Info("Migrating LFS object %s from %s to %s", meta.Oid, oldOidPath, newOidPath)
				// Ensure the new directory exists
				if err := os.MkdirAll(path.Dir(newOidPath), 0o750); err != nil {
					logger.Error("Could not create directory for %s: %v", newOidPath, err)
					return err
				}
				if err := os.Rename(oldOidPath, newOidPath); err != nil {
					logger.Error("Could not migrate %s to %s: %v", oldOidPath, newOidPath, err)
					return err
				}
			} else {
				logger.Info("LFS object %s needs migration from %s to %s", meta.Oid, oldOidPath, newOidPath)
			}
			return nil
		}, &git_model.IterateLFSMetaObjectsForRepoOptions{})
	})

	if err != nil {
		logger.Error("Error while migrating LFS object paths: %v", err)
		return err
	}

	if fix {
		logger.Info("Migrated %d LFS objects.", migratedCount)
	} else {
		logger.Info("Found %d LFS objects that need migration.", migratedCount)
	}

	return nil
}
