// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tempdir

import (
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type TempDir struct {
	// base is the base directory for temporary files, it must exist before accessing and won't be created automatically.
	// for example: base="/system-tmpdir", sub="gitea-tmp"
	base, sub string
}

func (td *TempDir) JoinPath(elems ...string) string {
	return filepath.Join(append([]string{td.base, td.sub}, elems...)...)
}

// MkdirAllSub works like os.MkdirAll, but the base directory must exist
func (td *TempDir) MkdirAllSub(dir string) (string, error) {
	if _, err := os.Stat(td.base); err != nil {
		return "", err
	}
	full := filepath.Join(td.base, td.sub, dir)
	if err := os.MkdirAll(full, os.ModePerm); err != nil {
		return "", err
	}
	return full, nil
}

func (td *TempDir) prepareDirWithPattern(elems ...string) (dir, pattern string, err error) {
	if _, err = os.Stat(td.base); err != nil {
		return "", "", err
	}
	dir, pattern = filepath.Split(filepath.Join(append([]string{td.base, td.sub}, elems...)...))
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", "", err
	}
	return dir, pattern, nil
}

// MkdirTempRandom works like os.MkdirTemp, the last path field is the "pattern"
func (td *TempDir) MkdirTempRandom(elems ...string) (string, func(), error) {
	dir, pattern, err := td.prepareDirWithPattern(elems...)
	if err != nil {
		return "", nil, err
	}
	dir, err = os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", nil, err
	}
	return dir, func() {
		if err := util.RemoveAll(dir); err != nil {
			log.Error("Failed to remove temp directory %s: %v", dir, err)
		}
	}, nil
}

// CreateTempFileRandom works like os.CreateTemp, the last path field is the "pattern"
func (td *TempDir) CreateTempFileRandom(elems ...string) (*os.File, func(), error) {
	dir, pattern, err := td.prepareDirWithPattern(elems...)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, nil, err
	}
	filename := f.Name()
	return f, func() {
		_ = f.Close()
		if err := util.Remove(filename); err != nil {
			log.Error("Unable to remove temporary file: %s: Error: %v", filename, err)
		}
	}, err
}

func (td *TempDir) RemoveOutdated(d time.Duration) {
	var remove func(path string)
	remove = func(path string) {
		entries, _ := os.ReadDir(path)
		for _, entry := range entries {
			full := filepath.Join(path, entry.Name())
			if entry.IsDir() {
				remove(full)
				_ = os.Remove(full)
				continue
			}
			info, err := entry.Info()
			if err == nil && time.Since(info.ModTime()) > d {
				_ = os.Remove(full)
			}
		}
	}
	remove(td.JoinPath(""))
}

// New create a new TempDir instance, "base" must be an existing directory,
// "sub" could be a multi-level directory and will be created if not exist
func New(base, sub string) *TempDir {
	return &TempDir{base: base, sub: sub}
}

func OsTempDir(sub string) *TempDir {
	return New(os.TempDir(), sub)
}
