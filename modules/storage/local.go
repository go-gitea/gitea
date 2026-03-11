// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var _ ObjectStorage = &LocalStorage{}

// LocalStorage represents a local files storage
type LocalStorage struct {
	ctx    context.Context
	dir    string
	tmpdir string
}

// NewLocalStorage returns a local files
func NewLocalStorage(ctx context.Context, config *setting.Storage) (ObjectStorage, error) {
	// prepare storage root path
	if !filepath.IsAbs(config.Path) {
		return nil, fmt.Errorf("LocalStorage config.Path should have been prepared by setting/storage.go and should be an absolute path, but not: %q", config.Path)
	}
	storageRoot := util.FilePathJoinAbs(config.Path)

	// prepare storage temporary path
	storageTmp := config.TemporaryPath
	if storageTmp == "" {
		storageTmp = filepath.Join(storageRoot, "tmp")
	}
	if !filepath.IsAbs(storageTmp) {
		return nil, fmt.Errorf("LocalStorage config.TemporaryPath should be an absolute path, but not: %q", config.TemporaryPath)
	}
	storageTmp = util.FilePathJoinAbs(storageTmp)

	// create the storage root if not exist
	log.Info("Creating new Local Storage at %s", storageRoot)
	if err := os.MkdirAll(storageRoot, os.ModePerm); err != nil {
		return nil, err
	}

	return &LocalStorage{
		ctx:    ctx,
		dir:    storageRoot,
		tmpdir: storageTmp,
	}, nil
}

func (l *LocalStorage) buildLocalPath(p string) string {
	return util.FilePathJoinAbs(l.dir, p)
}

// Open a file
func (l *LocalStorage) Open(path string) (Object, error) {
	return os.Open(l.buildLocalPath(path))
}

// Save a file
func (l *LocalStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	p := l.buildLocalPath(path)
	if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
		return 0, err
	}

	// Create a temporary file to save to
	if err := os.MkdirAll(l.tmpdir, os.ModePerm); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(l.tmpdir, "upload-*")
	if err != nil {
		return 0, err
	}
	tmpRemoved := false
	defer func() {
		if !tmpRemoved {
			_ = util.Remove(tmp.Name())
		}
	}()

	n, err := io.Copy(tmp, r)
	if err != nil {
		return 0, err
	}

	if err := tmp.Close(); err != nil {
		return 0, err
	}

	if err := util.Rename(tmp.Name(), p); err != nil {
		return 0, err
	}
	// Golang's tmp file (os.CreateTemp) always have 0o600 mode, so we need to change the file to follow the umask (as what Create/MkDir does)
	// but we don't want to make these files executable - so ensure that we mask out the executable bits
	if err := util.ApplyUmask(p, os.ModePerm&0o666); err != nil {
		return 0, err
	}

	tmpRemoved = true

	return n, nil
}

// Stat returns the info of the file
func (l *LocalStorage) Stat(path string) (os.FileInfo, error) {
	return os.Stat(l.buildLocalPath(path))
}

func (l *LocalStorage) deleteEmptyParentDirs(localFullPath string) {
	for parent := filepath.Dir(localFullPath); len(parent) > len(l.dir); parent = filepath.Dir(parent) {
		if err := os.Remove(parent); err != nil {
			// since the target file has been deleted, parent dir error is not related to the file deletion itself.
			break
		}
	}
}

// Delete deletes the file in storage and removes the empty parent directories (if possible)
func (l *LocalStorage) Delete(path string) error {
	localFullPath := l.buildLocalPath(path)
	err := util.Remove(localFullPath)
	l.deleteEmptyParentDirs(localFullPath)
	return err
}

// URL gets the redirect URL to a file
func (l *LocalStorage) URL(path, name, _ string, reqParams url.Values) (*url.URL, error) {
	return nil, ErrURLNotSupported
}

func (l *LocalStorage) normalizeWalkError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		// ignore it because the file may be deleted during the walk, and we don't care about it
		return nil
	}
	return err
}

// IterateObjects iterates across the objects in the local storage
func (l *LocalStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	dir := l.buildLocalPath(dirName)
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, errWalk error) error {
		if err := l.ctx.Err(); err != nil {
			return err
		}
		if errWalk != nil {
			return l.normalizeWalkError(errWalk)
		}
		if path == l.dir || d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(l.dir, path)
		if err != nil {
			return l.normalizeWalkError(err)
		}
		obj, err := os.Open(path)
		if err != nil {
			return l.normalizeWalkError(err)
		}
		defer obj.Close()
		return fn(filepath.ToSlash(relPath), obj)
	})
}

func init() {
	RegisterStorageType(setting.LocalStorageType, NewLocalStorage)
}
