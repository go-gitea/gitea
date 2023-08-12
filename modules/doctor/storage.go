// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

type commonStorageCheckOptions struct {
	storer     storage.ObjectStorage
	isOrphaned func(path string, obj storage.Object, stat fs.FileInfo) (bool, error)
	name       string
}

func commonCheckStorage(ctx context.Context, logger log.Logger, autofix bool, opts *commonStorageCheckOptions) error {
	totalCount, orphanedCount := 0, 0
	totalSize, orphanedSize := int64(0), int64(0)

	var pathsToDelete []string
	if err := opts.storer.IterateObjects("", func(p string, obj storage.Object) error {
		defer obj.Close()

		totalCount++
		stat, err := obj.Stat()
		if err != nil {
			return err
		}
		totalSize += stat.Size()

		orphaned, err := opts.isOrphaned(p, obj, stat)
		if err != nil {
			return err
		}
		if orphaned {
			orphanedCount++
			orphanedSize += stat.Size()
			if autofix {
				pathsToDelete = append(pathsToDelete, p)
			}
		}
		return nil
	}); err != nil {
		logger.Error("Error whilst iterating %s storage: %v", opts.name, err)
		return err
	}

	if orphanedCount > 0 {
		if autofix {
			var deletedNum int
			for _, p := range pathsToDelete {
				if err := opts.storer.Delete(p); err != nil {
					log.Error("Error whilst deleting %s from %s storage: %v", p, opts.name, err)
				} else {
					deletedNum++
				}
			}
			logger.Info("Deleted %d/%d orphaned %s(s)", deletedNum, orphanedCount, opts.name)
		} else {
			logger.Warn("Found %d/%d (%s/%s) orphaned %s(s)", orphanedCount, totalCount, base.FileSize(orphanedSize), base.FileSize(totalSize), opts.name)
		}
	} else {
		logger.Info("Found %d (%s) %s(s)", totalCount, base.FileSize(totalSize), opts.name)
	}
	return nil
}

type checkStorageOptions struct {
	All          bool
	Attachments  bool
	LFS          bool
	Avatars      bool
	RepoAvatars  bool
	RepoArchives bool
	Packages     bool
}

// checkStorage will return a doctor check function to check the requested storage types for "orphaned" stored object/files and optionally delete them
func checkStorage(opts *checkStorageOptions) func(ctx context.Context, logger log.Logger, autofix bool) error {
	return func(ctx context.Context, logger log.Logger, autofix bool) error {
		if err := storage.Init(); err != nil {
			logger.Error("storage.Init failed: %v", err)
			return err
		}

		if opts.Attachments || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.Attachments,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						exists, err := repo.ExistAttachmentsByUUID(ctx, stat.Name())
						return !exists, err
					},
					name: "attachment",
				}); err != nil {
				return err
			}
		}

		if opts.LFS || opts.All {
			if !setting.LFS.StartServer {
				logger.Info("LFS isn't enabled (skipped)")
				return nil
			}
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.LFS,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						// The oid of an LFS stored object is the name but with all the path.Separators removed
						oid := strings.ReplaceAll(path, "/", "")
						exists, err := git.ExistsLFSObject(ctx, oid)
						return !exists, err
					},
					name: "LFS file",
				}); err != nil {
				return err
			}
		}

		if opts.Avatars || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.Avatars,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						exists, err := user.ExistsWithAvatarAtStoragePath(ctx, path)
						return !exists, err
					},
					name: "avatar",
				}); err != nil {
				return err
			}
		}

		if opts.RepoAvatars || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.RepoAvatars,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						exists, err := repo.ExistsWithAvatarAtStoragePath(ctx, path)
						return !exists, err
					},
					name: "repo avatar",
				}); err != nil {
				return err
			}
		}

		if opts.RepoArchives || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.RepoAvatars,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						exists, err := repo.ExistsRepoArchiverWithStoragePath(ctx, path)
						if err == nil || errors.Is(err, util.ErrInvalidArgument) {
							// invalid arguments mean that the object is not a valid repo archiver and it should be removed
							return !exists, nil
						}
						return !exists, err
					},
					name: "repo archive",
				}); err != nil {
				return err
			}
		}

		if opts.Packages || opts.All {
			if !setting.Packages.Enabled {
				logger.Info("Packages isn't enabled (skipped)")
				return nil
			}
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.Packages,
					isOrphaned: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						key, err := packages_module.RelativePathToKey(path)
						if err != nil {
							// If there is an error here then the relative path does not match a valid package
							// Therefore it is orphaned by default
							return true, nil
						}

						exists, err := packages.ExistPackageBlobWithSHA(ctx, string(key))

						return !exists, err
					},
					name: "package blob",
				}); err != nil {
				return err
			}
		}

		return nil
	}
}

func init() {
	Register(&Check{
		Title:                      "Check if there are orphaned storage files",
		Name:                       "storages",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{All: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are orphaned attachments in storage",
		Name:                       "storage-attachments",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{Attachments: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are orphaned lfs files in storage",
		Name:                       "storage-lfs",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{LFS: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are orphaned avatars in storage",
		Name:                       "storage-avatars",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{Avatars: true, RepoAvatars: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are orphaned archives in storage",
		Name:                       "storage-archives",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{RepoArchives: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are orphaned package blobs in storage",
		Name:                       "storage-packages",
		IsDefault:                  false,
		Run:                        checkStorage(&checkStorageOptions{Packages: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}
