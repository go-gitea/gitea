// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// ErrLocalPathNotSupported represents an error that path is not supported
var ErrLocalPathNotSupported = errors.New("local path is not supported")
var _ ObjectStorage = &LocalStorage{}

// LocalStorageType is the type descriptor for local storage
const LocalStorageType Type = "local"

// LocalStorageConfig represents the configuration for a local storage
type LocalStorageConfig struct {
	Path          string `ini:"PATH"`
	TemporaryPath string `ini:"TEMPORARY_PATH"`
}

// LocalStorage represents a local files storage
type LocalStorage struct {
	ctx    context.Context
	dir    string
	tmpdir string
}

// NewLocalStorage returns a local files
func NewLocalStorage(ctx context.Context, cfg interface{}) (ObjectStorage, error) {
	configInterface, err := toConfig(LocalStorageConfig{}, cfg)
	if err != nil {
		return nil, err
	}
	config := configInterface.(LocalStorageConfig)

	log.Info("Creating new Local Storage at %s", config.Path)
	if err := os.MkdirAll(config.Path, os.ModePerm); err != nil {
		return nil, err
	}

	if config.TemporaryPath == "" {
		config.TemporaryPath = config.Path + "/tmp"
	}

	return &LocalStorage{
		ctx:    ctx,
		dir:    config.Path,
		tmpdir: config.TemporaryPath,
	}, nil
}

// Open a file
func (l *LocalStorage) Open(path string) (Object, error) {
	if !isLocalPathValid(path) {
		return nil, ErrLocalPathNotSupported
	}
	return os.Open(filepath.Join(l.dir, path))
}

// Save a file
func (l *LocalStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	if !isLocalPathValid(path) {
		return 0, ErrLocalPathNotSupported
	}

	p := filepath.Join(l.dir, path)
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

	tmpRemoved = true

	return n, nil
}

// Stat returns the info of the file
func (l *LocalStorage) Stat(path string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(l.dir, path))
}

func isLocalPathValid(p string) bool {
	a := path.Clean(p)
	if strings.HasPrefix(a, "../") || strings.HasPrefix(a, "..\\") {
		return false
	}
	return a == p
}

// Delete delete a file
func (l *LocalStorage) Delete(path string) error {
	if !isLocalPathValid(path) {
		return ErrLocalPathNotSupported
	}
	p := filepath.Join(l.dir, path)
	return util.Remove(p)
}

// URL gets the redirect URL to a file
func (l *LocalStorage) URL(path, name string) (*url.URL, error) {
	return nil, ErrURLNotSupported
}

// IterateObjects iterates across the objects in the local storage
func (l *LocalStorage) IterateObjects(fn func(path string, obj Object) error) error {
	return filepath.Walk(l.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-l.ctx.Done():
			return l.ctx.Err()
		default:
		}
		if path == l.dir {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(l.dir, path)
		if err != nil {
			return err
		}
		obj, err := os.Open(path)
		if err != nil {
			return err
		}
		defer obj.Close()
		return fn(relPath, obj)
	})
}

func init() {
	RegisterStorageType(LocalStorageType, NewLocalStorage)
}
