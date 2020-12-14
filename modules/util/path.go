// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"os"
	"path/filepath"
)

// EnsureAbsolutePath ensure that a path is absolute, making it
// relative to absoluteBase if necessary
func EnsureAbsolutePath(path string, absoluteBase string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(absoluteBase, path)
}

const notRegularFileMode os.FileMode = os.ModeDir | os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// GetDirectorySize returns the dumb disk consumption for a given path
func GetDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if info != nil && (info.Mode()&notRegularFileMode) == 0 {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// IsDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func IsDir(dir string) (bool, error) {
	f, err := os.Stat(dir)
	if err == nil {
		return f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsFile returns true if given path is a file,
// or returns false when it's a directory or does not exist.
func IsFile(filePath string) (bool, error) {
	f, err := os.Stat(filePath)
	if err == nil {
		return !f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || os.IsExist(err) {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
