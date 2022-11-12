// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/repo"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
)

type commonStorageCheckOptions struct {
	storer       storage.ObjectStorage
	isAssociated func(path string, obj storage.Object, stat fs.FileInfo) (bool, error)
	name         string
}

func commonCheckStorage(ctx context.Context, logger log.Logger, autofix bool, opts *commonStorageCheckOptions) error {
	totalCount, unassociatedCount := 0, 0
	totalSize, unassociatedSize := int64(0), int64(0)

	var pathsToDelete []string
	if err := opts.storer.IterateObjects(func(p string, obj storage.Object) error {
		defer obj.Close()

		totalCount++
		stat, err := obj.Stat()
		if err != nil {
			return err
		}
		totalSize += stat.Size()

		associated, err := opts.isAssociated(p, obj, stat)
		if err != nil {
			return err
		}
		if !associated {
			unassociatedCount++
			unassociatedSize += stat.Size()
			if autofix {
				pathsToDelete = append(pathsToDelete, p)
			}
		}
		return nil
	}); err != nil {
		logger.Error("Error whilst iterating %s storage: %v", opts.name, err)
		return err
	}

	if unassociatedCount > 0 {
		if autofix {
			var deletedNum int
			for _, p := range pathsToDelete {
				if err := opts.storer.Delete(p); err != nil {
					log.Error("Error whilst deleting %s from %s storage: %v", p, opts.name, err)
				} else {
					deletedNum++
				}
			}
			logger.Info("Deleted %d/%d unassociated %s(s)", deletedNum, unassociatedCount, opts.name)
		} else {
			logger.Warn("Found %d/%d (%s/%s) unassociated %s(s)", unassociatedCount, totalCount, base.FileSize(unassociatedSize), base.FileSize(totalSize), opts.name)
		}
	} else {
		logger.Info("Found %d (%s) %s(s)", totalCount, base.FileSize(totalSize), opts.name)
	}
	return nil
}

type storageCheckOptions struct {
	All          bool
	Attachments  bool
	LFS          bool
	Avatars      bool
	RepoAvatars  bool
	RepoArchives bool
	Packages     bool
}

func checkStorage(opts *storageCheckOptions) func(ctx context.Context, logger log.Logger, autofix bool) error {
	return func(ctx context.Context, logger log.Logger, autofix bool) error {
		if err := storage.Init(); err != nil {
			logger.Error("storage.Init failed: %v", err)
			return err
		}

		if opts.Attachments || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.Attachments,
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						return repo_model.ExistAttachmentsByUUID(ctx, stat.Name())
					},
					name: "attachment",
				}); err != nil {
				return err
			}
		}

		if opts.LFS || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.LFS,
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						// The oid of an LFS stored object is the name but with all the path.Separators removed
						oid := strings.ReplaceAll(path, "/", "")

						return git.LFSObjectIsAssociated(ctx, oid)
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
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						return user.ExistUserWithAvatar(ctx, path)
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
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						return repo.ExistRepoWithAvatar(ctx, path)
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
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						has, err := repo.ExistsRepoArchiverWithStoragePath(ctx, path)
						if err == nil || errors.Is(err, util.ErrInvalidArgument) {
							// invalid arguments mean that the object is not a valid repo archiver and it should be removed
							return has, nil
						}
						return has, err
					},
					name: "repo archive",
				}); err != nil {
				return err
			}
		}

		if opts.Packages || opts.All {
			if err := commonCheckStorage(ctx, logger, autofix,
				&commonStorageCheckOptions{
					storer: storage.Packages,
					isAssociated: func(path string, obj storage.Object, stat fs.FileInfo) (bool, error) {
						parts := strings.SplitN(path, "/", 3)
						if len(parts) != 3 || len(parts[0]) != 2 || len(parts[1]) != 2 || len(parts[2]) < 4 || parts[0]+parts[1] != parts[2][0:4] {
							return false, nil
						}

						return packages.ExistPackageBlobWithSHA(ctx, parts[2])
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
		Title:                      "Check if there are unassociated storage files",
		Name:                       "storages",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{All: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are unassociated attachments in storage",
		Name:                       "storage-attachments",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{Attachments: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are unassociated lfs files in storage",
		Name:                       "storage-lfs",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{LFS: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are unassociated avatars in storage",
		Name:                       "storage-avatars",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{Avatars: true, RepoAvatars: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are unassociated archives in storage",
		Name:                       "storage-archives",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{RepoArchives: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})

	Register(&Check{
		Title:                      "Check if there are unassociated package blobs in storage",
		Name:                       "storage-packages",
		IsDefault:                  false,
		Run:                        checkStorage(&storageCheckOptions{Packages: true}),
		AbortIfFailed:              false,
		SkipDatabaseInitialization: false,
		Priority:                   1,
	})
}
