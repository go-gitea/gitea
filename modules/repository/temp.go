// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
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
		return "", fmt.Errorf("Failed to create localcopypath directory %s: %w", LocalCopyPath(), err)
	}
	basePath, err := os.MkdirTemp(LocalCopyPath(), prefix+".git")
	if err != nil {
		log.Error("Unable to create temporary directory: %s-*.git (%v)", prefix, err)
		return "", fmt.Errorf("Failed to create dir %s-*.git: %w", prefix, err)
	}
	return basePath, nil
}

// RemoveTemporaryPath removes the temporary path
func RemoveTemporaryPath(basePath string) error {
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		return util.RemoveAll(basePath)
	}
	return nil
}
