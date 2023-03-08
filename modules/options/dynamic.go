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

	customDir := path.Join(setting.CustomPath, "options", name)

	isDir, err := util.IsDir(customDir)
	if err != nil {
		return []string{}, fmt.Errorf("Unabe to check if custom directory %s is a directory. %w", customDir, err)
	}
	if isDir {
		files, err := util.StatDir(customDir, true)
		if err != nil {
			return []string{}, fmt.Errorf("Failed to read custom directory. %w", err)
		}

		result = append(result, files...)
	}

	staticDir := path.Join(setting.StaticRootPath, "options", name)

	isDir, err = util.IsDir(staticDir)
	if err != nil {
		return []string{}, fmt.Errorf("unable to check if static directory %s is a directory. %w", staticDir, err)
	}
	if isDir {
		files, err := util.StatDir(staticDir, true)
		if err != nil {
			return []string{}, fmt.Errorf("Failed to read static directory. %w", err)
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
