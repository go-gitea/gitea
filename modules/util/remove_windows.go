// +build windows,!go1.7

// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

// RemoveAll files from path on windows
// workaround for Go not being able to remove read-only files/folders: https://github.com/golang/go/issues/9606
// this bug should be fixed on Go 1.7, so the workaround should be removed when Gitea don't support Go 1.6 anymore:
// https://github.com/golang/go/commit/2ffb3e5d905b5622204d199128dec06cefd57790
func RemoveAll(path string) error {
	path = strings.Replace(path, "/", "\\", -1)
	return exec.Command("cmd", "/C", "rmdir", "/S", "/Q", path).Run()
}
