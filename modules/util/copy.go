// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"github.com/unknwon/com"
)

// CopyFile copies file from source to target path.
func CopyFile(src, dest string) error {
	return com.Copy(src, dest)
}

// CopyDir copy files recursively from source to target directory.
// It returns error when error occurs in underlying functions.
func CopyDir(srcPath, destPath string) error {
	return com.CopyDir(srcPath, destPath)
}
