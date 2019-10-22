// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"io/ioutil"
	"path"

	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

var (
	directories = make(directorySet)
)

// Dir returns all files from static or custom directory.
func Dir(name string) ([]string, error) {
	if directories.Filled(name) {
		return directories.Get(name), nil
	}

	var (
		result []string
	)

	customDir := path.Join(setting.CustomPath, "options", name)

	if com.IsDir(customDir) {
		files, err := com.StatDir(customDir, true)

		if err != nil {
			return []string{}, fmt.Errorf("Failed to read custom directory. %v", err)
		}

		result = append(result, files...)
	}

	staticDir := path.Join(setting.StaticRootPath, "options", name)

	if com.IsDir(staticDir) {
		files, err := com.StatDir(staticDir, true)

		if err != nil {
			return []string{}, fmt.Errorf("Failed to read static directory. %v", err)
		}

		result = append(result, files...)
	}

	return directories.AddAndGet(name, result), nil
}

// Locale reads the content of a specific locale from static or custom path.
func Locale(name string) ([]byte, error) {
	return fileFromDir(path.Join("locale", name))
}

// Readme reads the content of a specific readme from static or custom path.
func Readme(name string) ([]byte, error) {
	return fileFromDir(path.Join("readme", name))
}

// Gitignore reads the content of a specific gitignore from static or custom path.
func Gitignore(name string) ([]byte, error) {
	return fileFromDir(path.Join("gitignore", name))
}

// License reads the content of a specific license from static or custom path.
func License(name string) ([]byte, error) {
	return fileFromDir(path.Join("license", name))
}

// Labels reads the content of a specific labels from static or custom path.
func Labels(name string) ([]byte, error) {
	return fileFromDir(path.Join("label", name))
}

// fileFromDir is a helper to read files from static or custom path.
func fileFromDir(name string) ([]byte, error) {
	customPath := path.Join(setting.CustomPath, "options", name)

	if com.IsFile(customPath) {
		return ioutil.ReadFile(customPath)
	}

	staticPath := path.Join(setting.StaticRootPath, "options", name)

	if com.IsFile(staticPath) {
		return ioutil.ReadFile(staticPath)
	}

	return []byte{}, fmt.Errorf("Asset file does not exist: %s", name)
}
