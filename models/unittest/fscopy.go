// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

type syncSource struct {
	entries []string // util.ListDirRecursively appends a slash to the directory names
	infos   map[string]os.FileInfo
}

var (
	syncSourcesMu sync.Mutex
	syncSources   = map[string]*syncSource{}
)

func loadSyncSource(srcPath string) (*syncSource, error) {
	syncSourcesMu.Lock()
	defer syncSourcesMu.Unlock()
	if src, ok := syncSources[srcPath]; ok {
		return src, nil
	}
	entries, err := util.ListDirRecursively(srcPath, &util.ListDirOptions{IncludeDir: true})
	if err != nil {
		return nil, err
	}
	src := &syncSource{entries: entries, infos: make(map[string]os.FileInfo, len(entries))}
	for _, entry := range entries {
		if src.infos[entry], err = os.Stat(filepath.Join(srcPath, entry)); err != nil {
			return nil, err
		}
	}
	syncSources[srcPath] = src
	return src, nil
}

// syncFile copies the source file unless the destination has the same size, modtime and mode
func syncFile(srcInfo os.FileInfo, srcPath, destPath string) error {
	dest, err := os.Stat(destPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && srcInfo.Size() == dest.Size() && srcInfo.ModTime().Equal(dest.ModTime()) && srcInfo.Mode() == dest.Mode() {
		return nil
	}
	return util.CopyFile(srcPath, destPath)
}

// SyncDirs synchronizes files recursively from source to target directory.
// The source directory is only listed once per process, it must not change.
// It returns error when error occurs in underlying functions.
func SyncDirs(srcPath, destPath string) error {
	destPath = filepath.Clean(destPath)
	destPathAbs, err := filepath.Abs(destPath)
	if err != nil {
		return err
	}
	devDataPathAbs, err := filepath.Abs(filepath.Join(setting.GetGiteaTestSourceRoot(), "data"))
	if err != nil {
		return err
	}
	if strings.HasPrefix(destPathAbs+string(filepath.Separator), devDataPathAbs+string(filepath.Separator)) {
		return errors.New("destination path should not be inside Gitea data directory, otherwise your data for dev mode will be removed")
	}

	src, err := loadSyncSource(srcPath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// the keep file is used to keep the directory in a git repository, it doesn't need to be synced
	const keepFile = ".keep"

	// find and delete all untracked files
	destFiles, err := util.ListDirRecursively(destPath, &util.ListDirOptions{IncludeDir: true})
	if err != nil {
		return err
	}
	for _, destFile := range destFiles {
		if _, ok := src.infos[destFile]; !ok || filepath.Base(destFile) == keepFile {
			if err = os.RemoveAll(filepath.Join(destPath, destFile)); err != nil {
				return err
			}
		}
	}

	// sync src files to dest
	for _, srcFile := range src.entries {
		destFilePath := filepath.Join(destPath, srcFile)
		if strings.HasSuffix(srcFile, "/") {
			err = os.MkdirAll(destFilePath, os.ModePerm)
		} else if filepath.Base(destFilePath) != keepFile {
			err = syncFile(src.infos[srcFile], filepath.Join(srcPath, srcFile), destFilePath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
