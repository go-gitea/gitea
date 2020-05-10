// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// LocalCopyPath returns the local repository temporary copy path.
func LocalCopyPath() string {
	if filepath.IsAbs(setting.Repository.Local.LocalCopyPath) {
		return setting.Repository.Local.LocalCopyPath
	}
	return path.Join(setting.AppDataPath, setting.Repository.Local.LocalCopyPath)
}

// CreateTemporaryPath creates a temporary path
func CreateTemporaryPath(prefix string) (string, error) {
	if err := os.MkdirAll(LocalCopyPath(), os.ModePerm); err != nil {
		log.Error("Unable to create localcopypath directory: %s (%v)", LocalCopyPath(), err)
		return "", fmt.Errorf("Failed to create localcopypath directory %s: %v", LocalCopyPath(), err)
	}
	basePath, err := ioutil.TempDir(LocalCopyPath(), prefix+".git")
	if err != nil {
		log.Error("Unable to create temporary directory: %s-*.git (%v)", prefix, err)
		return "", fmt.Errorf("Failed to create dir %s-*.git: %v", prefix, err)

	}
	return basePath, nil
}

// RemoveTemporaryPath removes the temporary path
func RemoveTemporaryPath(basePath string) error {
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		return os.RemoveAll(basePath)
	}
	return nil
}
