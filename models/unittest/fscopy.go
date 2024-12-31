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
// It returns error when error occurs in underlying functions.
func SyncDirs(srcPath, destPath string) error {
	err := os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// find and delete all untracked files
	destFiles, err := util.ListDirRecursively(destPath, &util.ListDirOptions{IncludeDir: true})
	if err != nil {
		return err
	}
	for _, destFile := range destFiles {
		destFilePath := filepath.Join(destPath, destFile)
		if _, err = os.Stat(filepath.Join(srcPath, destFile)); err != nil {
			if os.IsNotExist(err) {
				// if src file does not exist, remove dest file
				if err = os.RemoveAll(destFilePath); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// sync src files to dest
	srcFiles, err := util.ListDirRecursively(srcPath, &util.ListDirOptions{IncludeDir: true})
	if err != nil {
		return err
	}
	for _, srcFile := range srcFiles {
		destFilePath := filepath.Join(destPath, srcFile)
		// util.ListDirRecursively appends a slash to the directory name
		if strings.HasSuffix(srcFile, "/") {
			err = os.MkdirAll(destFilePath, os.ModePerm)
		} else {
			err = Sync(filepath.Join(srcPath, srcFile), destFilePath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
