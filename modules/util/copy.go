// Copyright 2013 com authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.
//
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// CopyFile copies file from source to target path.
func CopyFile(src, dest string) error {
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
		// NOTE: os.Chmod and os.Chtimes don't recoganize symbolic link,
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

// CopyDir copy files recursively from source to target directory.
//
// The filter accepts a function that process the path info.
// and should return true for need to filter.
//
// It returns error when error occurs in underlying functions.
func CopyDir(srcPath, destPath string, filters ...func(filePath string) bool) error {
	// Check if target directory exists.
	targetExist, err := IsExist(destPath)
	if err != nil {
		return err
	}
	if targetExist {
		return fmt.Errorf("file or directory alreay exists: '%s'", destPath)
	}

	err = os.MkdirAll(destPath, os.ModePerm)
	if err != nil {
		return err
	}

	// Gather directory info.
	infos, err := StatDir(srcPath, true)
	if err != nil {
		return err
	}

	var filter func(filePath string) bool
	if len(filters) > 0 {
		filter = filters[0]
	}

	for _, info := range infos {
		if filter != nil && filter(info) {
			continue
		}

		curPath := path.Join(destPath, info)
		if strings.HasSuffix(info, "/") {
			err = os.MkdirAll(curPath, os.ModePerm)
		} else {
			err = CopyFile(path.Join(srcPath, info), curPath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
