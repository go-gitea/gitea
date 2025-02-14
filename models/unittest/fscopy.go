// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// SyncFile synchronizes the two files. This is skipped if both files
// exist and the size, modtime, and mode match.
func SyncFile(srcPath, destPath string) error {
	dest, err := os.Stat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			return util.CopyFile(srcPath, destPath)
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

	return util.CopyFile(srcPath, destPath)
}

// SyncDirs synchronizes files recursively from source to target directory.
// It returns error when error occurs in underlying functions.
func SyncDirs(srcPath, destPath string) error {
	err := os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// the keep file is used to keep the directory in a git repository, it doesn't need to be synced
	// and go-git doesn't work with the ".keep" file (it would report errors like "ref is empty")
	const keepFile = ".keep"

	// find and delete all untracked files
	destFiles, err := util.ListDirRecursively(destPath, &util.ListDirOptions{IncludeDir: true})
	if err != nil {
		return err
	}
	for _, destFile := range destFiles {
		destFilePath := filepath.Join(destPath, destFile)
		shouldRemove := filepath.Base(destFilePath) == keepFile
		if _, err = os.Stat(filepath.Join(srcPath, destFile)); err != nil {
			if os.IsNotExist(err) {
				shouldRemove = true
			} else {
				return err
			}
		}
		// if src file does not exist, remove dest file
		if shouldRemove {
			if err = os.RemoveAll(destFilePath); err != nil {
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
		} else if filepath.Base(destFilePath) != keepFile {
			err = SyncFile(filepath.Join(srcPath, srcFile), destFilePath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
