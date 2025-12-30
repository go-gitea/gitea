// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"io/fs"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var ErrRepoStoreConfig = util.NewInvalidArgumentErrorf("invalid repository store configuration")

func RepoStoreStat() error {
	if setting.RepoRootPath == "" {
		return ErrRepoStoreConfig
	}
	_, err := os.Stat(setting.RepoRootPath)
	return err
}

func SyncLocalToRepoStore(localDir string) error {
	return util.SyncDirs(localDir, setting.RepoRootPath)
}

func RemoveRepoStore() error {
	return util.RemoveAll(setting.RepoRootPath)
}

func RemoveRepoStoreDir(dirName string) error {
	return util.RemoveAll(filepath.Join(setting.RepoRootPath, dirName))
}

func RenameRepoStoreDir(oldDirName, newDirName string) error {
	oldPath := filepath.Join(setting.RepoRootPath, oldDirName)
	newPath := filepath.Join(setting.RepoRootPath, newDirName)
	return util.Rename(oldPath, newPath)
}

func WalkRepoStoreDirs(relativeDir string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(filepath.Join(setting.RepoRootPath, relativeDir), func(path string, d os.DirEntry, err error) error {
		p, err1 := filepath.Rel(relativeDir, path)
		if err1 != nil {
			return err1
		}
		return fn(p, d, err)
	})
}
