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
	IsConfigured() bool
	CheckStats() error
	IsExist(path string) (bool, error)
	// MakeDir(repoRelPath string) error
	MakeDir(dir string, perm os.FileMode) error
	MakeRepoDir(repoRelPath string) error
	RemoveAll(path string) error
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

func (l *LocalStorage) IsConfigured() bool {
	return len(l.repoRootPath) != 0
}

func (l *LocalStorage) CheckStats() error {
	_, err := os.Stat(l.repoRootPath)
	return err
}

func (l *LocalStorage) IsExist(path string) (bool, error) {
	return util.IsExist(path)
}

func (l *LocalStorage) MakeDir(dir string, perm os.FileMode) error {
	return os.MkdirAll(l.absPath(dir), perm)
}

func (l *LocalStorage) MakeRepoDir(repoRelPath string) error {
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "pack"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "info"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "heads"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "tag"), 0o755)
	return nil
}

func (l *LocalStorage) RemoveAll(path string) error {
	// TODO: removeAllWithRetry(l.absPath(path))
	return os.RemoveAll(l.absPath(path))
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

func IsConfigured() bool {
	return getStorage().IsConfigured()
}

func CheckStats() error {
	return getStorage().CheckStats()
}

func IsExist(path string) (bool, error) {
	return getStorage().IsExist(path)
}

func MakeDir(dir string, perm os.FileMode) error {
	return getStorage().MakeDir(dir, perm)
}

func MakeRepoDir(repoRelPath string) error {
	return getStorage().MakeRepoDir(repoRelPath)
}

func RemoveAll(p string) error {
	return getStorage().RemoveAll(p)
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
