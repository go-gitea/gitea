// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tempdir

import (
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

type TempDir struct {
	base, sub string
}

func (td *TempDir) JoinPath(elems ...string) string {
	return filepath.Join(append([]string{td.base, td.sub}, elems...)...)
}

func (td *TempDir) Mkdir(dir string) (string, error) {
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

func (td *TempDir) RemoveOutdated() {
	// TODO: remove the out-dated temp files
	log.Error("TODO: remove the out-dated temp files, not implemented yet")
}

func OsTempDir(sub string) *TempDir {
	return &TempDir{base: os.TempDir(), sub: sub}
}

func New(base, sub string) *TempDir {
	return &TempDir{base: base, sub: sub}
}
