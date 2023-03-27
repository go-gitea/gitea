// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package options

import (
	"code.gitea.io/gitea/modules/setting"
)

// Dir returns all files from static or custom directory.
func Dir(name string) ([]string, error) {
	if directories.Filled(name) {
		return directories.Get(name), nil
	}

	result, err := listLocalDirIfExist([]string{setting.CustomPath, setting.StaticRootPath}, "options", name)
	if err != nil {
		return nil, err
	}

	return directories.AddAndGet(name, result), nil
}

// fileFromOptionsDir is a helper to read files from custom or static path.
func fileFromOptionsDir(elems ...string) ([]byte, error) {
	return readLocalFile([]string{setting.CustomPath, setting.StaticRootPath}, "options", elems...)
}

// IsDynamic will return false when using embedded data (-tags bindata)
func IsDynamic() bool {
	return true
}
