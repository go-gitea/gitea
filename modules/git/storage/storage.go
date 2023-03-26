// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"io/fs"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type Storage interface {
	IsValid() bool
	MakeDir(repoRelPath string) error
	RemoveAllRepos() error
	ReadDir(owner string) ([]fs.DirEntry, error)
	CopyDir(source, target string) error
	Rename(oldPath, newPath string) error
}

type LocalStorage struct {
	repoRootPath string
}

var _ Storage = &LocalStorage{}

func (l *LocalStorage) absPath(relPath string) string {
	return filepath.Join(l.repoRootPath, relPath)
}

func (l *LocalStorage) IsValid() bool {
	return len(l.repoRootPath) != 0
}

func (l *LocalStorage) MakeDir(repoRelPath string) error {
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "pack"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "info"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "heads"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "tag"), 0o755)
	return nil
}

func (l *LocalStorage) RemoveAllRepos() error {
	// removeAllWithRetry(setting.RepoRootPath)
	return os.RemoveAll(l.absPath(""))
}

func (l *LocalStorage) ReadDir(owner string) ([]fs.DirEntry, error) {
	return os.ReadDir(l.absPath(owner))
}

func (l *LocalStorage) CopyDir(source, target string) error {
	return util.CopyDir(source, l.absPath(target))
}

func (l *LocalStorage) Rename(oldPath, newPath string) error {
	return util.Rename(l.absPath(oldPath), l.absPath(newPath))
}

func getStorage() Storage {
	return &LocalStorage{
		repoRootPath: setting.RepoRootPath,
	}
}

func IsReposValid() bool {
	return getStorage().IsValid()
}

func MakeDir(repoRelPath string) error {
	return getStorage().MakeDir(repoRelPath)
}

func RemoveAllRepos() error {
	return getStorage().RemoveAllRepos()
}

func ReadDir(owner string) ([]fs.DirEntry, error) {
	return getStorage().ReadDir(owner)
}

func CopyDir(source, target string) error {
	return getStorage().CopyDir(source, target)
}

func Rename(oldPath, newPath string) error {
	return getStorage().Rename(oldPath, newPath)
}
