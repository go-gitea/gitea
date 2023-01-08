// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io/fs"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
)

type Storage interface {
	MakeDir(repoRelPath string) error
	RemoveAll() error
	ReadDir(owner string) ([]fs.DirEntry, error)
}

type LocalStorage struct{}

var _ Storage = &LocalStorage{}

func (l *LocalStorage) MakeDir(repoRelPath string) error {
	_ = os.MkdirAll(filepath.Join(absPath(repoRelPath), "objects", "pack"), 0o755)
	_ = os.MkdirAll(filepath.Join(absPath(repoRelPath), "objects", "info"), 0o755)
	_ = os.MkdirAll(filepath.Join(absPath(repoRelPath), "refs", "heads"), 0o755)
	_ = os.MkdirAll(filepath.Join(absPath(repoRelPath), "refs", "tag"), 0o755)
	return nil
}

func (l *LocalStorage) RemoveAll() error {
	return os.RemoveAll(setting.RepoRootPath)
}

func (l *LocalStorage) ReadDir(owner string) ([]fs.DirEntry, error) {
	return os.ReadDir(absPath(owner))
}

func GetStorage() Storage {
	return &LocalStorage{}
}
