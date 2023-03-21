// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package options

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var directories = make(directorySet)

// Dir returns all files from static or custom directory.
func Dir(name string) ([]string, error) {
	if directories.Filled(name) {
		return directories.Get(name), nil
	}

	var result []string

	for _, dir := range []string{
		util.SafeFilePathAbs(setting.CustomPath, "options", name),     // custom dir
		util.SafeFilePathAbs(setting.StaticRootPath, "options", name), // static dir
	} {
		files, err := statDirIfExist(dir)
		if err != nil {
			return nil, err
		}
		result = append(result, files...)
	}

	return directories.AddAndGet(name, result), nil
}

// fileFromOptionsDir is a helper to read files from static or custom path.
func fileFromOptionsDir(elems ...string) ([]byte, error) {
	return readFileFromLocal([]string{setting.CustomPath, setting.StaticRootPath}, "options", elems...)
}

// IsDynamic will return false when using embedded data (-tags bindata)
func IsDynamic() bool {
	return true
}
