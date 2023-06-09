// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type Storage interface {
	Configuration() string // information for configuration, json format
	CheckStats() error
	IsExist(path string) (bool, error)
	IsDir(path string) (bool, error)
	// MakeDir(repoRelPath string) error
	MakeDir(dir string, perm os.FileMode) error
	MakeRepoDir(repoRelPath string) error
	RemoveAll(path string) error
	ReadDir(owner string) ([]fs.DirEntry, error)
	UploadDir(localSource, targetRelPath string) error
	Rename(oldPath, newPath string) error
}

type LocalSingleStorage struct {
	repoRootPath string
}

func (l *LocalSingleStorage) Configuration() string {
	return "{RepoRootPath: " + l.repoRootPath + "}"
}

func (l *LocalSingleStorage) absPath(relPath string) string {
	return filepath.Join(l.repoRootPath, relPath)
}

func (l *LocalSingleStorage) CheckStats() error {
	// Check if l.repoRootPath exists. It could be the case that it doesn't exist, this can happen when
	// `[repository]` `ROOT` is a relative path and $GITEA_WORK_DIR isn't passed to the SSH connection.
	if _, err := os.Stat(l.repoRootPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory `[repository].ROOT` %q was not found, please check if $GITEA_WORK_DIR is passed to the SSH connection or make `[repository].ROOT` an absolute value",
				l.repoRootPath)
		}
		return fmt.Errorf("directory `[repository].ROOT` %q is inaccessible. err: %v",
			l.repoRootPath, err)
	}

	return nil
}

func (l *LocalSingleStorage) IsExist(path string) (bool, error) {
	return util.IsExist(l.absPath(path))
}

func (l *LocalSingleStorage) IsDir(path string) (bool, error) {
	return util.IsDir(l.absPath(path))
}

func (l *LocalSingleStorage) MakeDir(dir string, perm os.FileMode) error {
	return os.MkdirAll(l.absPath(dir), perm)
}

func (l *LocalSingleStorage) MakeRepoDir(repoRelPath string) error {
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "pack"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "objects", "info"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "heads"), 0o755)
	_ = os.MkdirAll(filepath.Join(l.absPath(repoRelPath), "refs", "tag"), 0o755)
	return nil
}

func (l *LocalSingleStorage) RemoveAll(path string) error {
	// TODO: removeAllWithRetry(l.absPath(path))
	return os.RemoveAll(l.absPath(path))
}

func (l *LocalSingleStorage) ReadDir(owner string) ([]fs.DirEntry, error) {
	return os.ReadDir(l.absPath(owner))
}

func (l *LocalSingleStorage) UploadDir(localSource, targetRelPath string) error {
	return util.CopyDir(localSource, l.absPath(targetRelPath))
}

func (l *LocalSingleStorage) Rename(oldPath, newPath string) error {
	return util.Rename(l.absPath(oldPath), l.absPath(newPath))
}

var storage Storage

func Init() error {
	storage = &LocalSingleStorage{
		repoRootPath: setting.RepoRootPath,
	}

	return storage.CheckStats()
}

func getStorage() Storage {
	return storage
}

func Configuration() string {
	return getStorage().Configuration()
}

func CheckStats() error {
	return getStorage().CheckStats()
}

func IsExist(path string) (bool, error) {
	return getStorage().IsExist(path)
}

func IsDir(path string) (bool, error) {
	return getStorage().IsDir(path)
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

func UploadDir(source, target string) error {
	return getStorage().UploadDir(source, target)
}

func Rename(oldPath, newPath string) error {
	return getStorage().Rename(oldPath, newPath)
}

// UserRelPath returns the path relative path of user repositories.
func UserRelPath(userName string) string {
	return strings.ToLower(userName)
}

// RepoRelPath returns repository relative path by given user and repository name.
func RepoRelPath(userName, repoName string) string {
	return path.Join(strings.ToLower(userName), strings.ToLower(repoName)+".git")
}

// WikiRelPath returns wiki repository relative path by given user and repository name.
func WikiRelPath(userName, repoName string) string {
	return path.Join(strings.ToLower(userName), strings.ToLower(repoName)+".wiki.git")
}
