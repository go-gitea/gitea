// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// Copy copies file from source to target path.
func Copy(src, dest string) error {
	// Gather file information to set back later.
	si, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Handle symbolic link.
	if si.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		// NOTE: os.Chmod and os.Chtimes don't recognize symbolic link,
		// which will lead "no such file or directory" error.
		return os.Symlink(target, dest)
	}

	return util.CopyFile(src, dest)
}

// Sync synchronizes the two files. This is skipped if both files
// exist and the size, modtime, and mode match.
func Sync(srcPath, destPath string) error {
	dest, err := os.Stat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Copy(srcPath, destPath)
		}
		return err
	}

	src, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	if src.Size() == dest.Size() &&
		src.ModTime() == dest.ModTime() &&
		src.Mode() == dest.Mode() {
		return nil
	}

	return Copy(srcPath, destPath)
}

// SyncDirs synchronizes files recursively from source to target directory.
//
// It returns error when error occurs in underlying functions.
func SyncDirs(srcPath, destPath string) error {
	err := os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// find and delete all untracked files
	files, err := util.StatDir(destPath, true)
	if err != nil {
		return err
	}

	for _, file := range files {
		destFilePath := filepath.Join(destPath, file)
		if _, err = os.Stat(filepath.Join(srcPath, file)); err != nil {
			if os.IsNotExist(err) {
				// TODO: why delete? it should happen because the file list is just queried above, why not exist?
				if err := os.RemoveAll(destFilePath); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// Gather directory info.
	files, err = util.StatDir(srcPath, true)
	if err != nil {
		return err
	}

	for _, file := range files {
		destFilePath := filepath.Join(destPath, file)
		if strings.HasSuffix(file, "/") {
			err = os.MkdirAll(destFilePath, os.ModePerm)
		} else {
			err = Sync(filepath.Join(srcPath, file), destFilePath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
