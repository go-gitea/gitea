// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package options

import (
	"fmt"
	"os"
	"path"

	"code.gitea.io/gitea/modules/log"
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
		path.Join(setting.CustomPath, "options", name),     // custom dir
		path.Join(setting.StaticRootPath, "options", name), // static dir
	} {
		files, err := statDirIfExist(dir)
		if err != nil {
			return nil, err
		}
		result = append(result, files...)
	}

	return directories.AddAndGet(name, result), nil
}

// fileFromDir is a helper to read files from static or custom path.
func fileFromDir(name string) ([]byte, error) {
	customPath := path.Join(setting.CustomPath, "options", name)

	isFile, err := util.IsFile(customPath)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", customPath, err)
	}
	if isFile {
		return os.ReadFile(customPath)
	}

	staticPath := path.Join(setting.StaticRootPath, "options", name)

	isFile, err = util.IsFile(staticPath)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", staticPath, err)
	}
	if isFile {
		return os.ReadFile(staticPath)
	}

	return []byte{}, fmt.Errorf("Asset file does not exist: %s", name)
}

// IsDynamic will return false when using embedded data (-tags bindata)
func IsDynamic() bool {
	return true
}
