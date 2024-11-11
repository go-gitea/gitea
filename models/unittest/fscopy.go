// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package unittest

import (
	"io"
	"os"
	"path"
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

	sr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sr.Close()

	dw, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer dw.Close()

	if _, err = io.Copy(dw, sr); err != nil {
		return err
	}

	// Set back file information.
	if err = os.Chtimes(dest, si.ModTime(), si.ModTime()); err != nil {
		return err
	}
	return os.Chmod(dest, si.Mode())
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
	infos, err := util.StatDir(destPath, true)
	if err != nil {
		return err
	}

	for _, info := range infos {
		curPath := path.Join(destPath, info)

		if _, err := os.Stat(path.Join(srcPath, info)); err != nil {
			if os.IsNotExist(err) {
				// Delete
				if err := os.RemoveAll(curPath); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	// Gather directory info.
	infos, err = util.StatDir(srcPath, true)
	if err != nil {
		return err
	}

	for _, info := range infos {
		curPath := path.Join(destPath, info)
		if strings.HasSuffix(info, "/") {
			err = os.MkdirAll(curPath, os.ModePerm)
		} else {
			err = Sync(path.Join(srcPath, info), curPath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
