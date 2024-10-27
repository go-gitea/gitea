// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// localCopyPath returns the local repository temporary copy path.
func localCopyPath() string {
	return filepath.Join(setting.TempDir(), "local-repo")
}

func CleanUpTemporaryPaths() {
	if err := util.RemoveAll(localCopyPath()); err != nil {
		log.Error("Unable to remove local repository temporary copy path: %s (%v)", localCopyPath(), err)
	}
}

// CreateTemporaryPath creates a temporary path
func CreateTemporaryPath(prefix string) (string, func(), error) {
	if err := os.MkdirAll(localCopyPath(), os.ModePerm); err != nil {
		log.Error("Unable to create localcopypath directory: %s (%v)", localCopyPath(), err)
		return "", func() {}, fmt.Errorf("failed to create localcopypath directory %s: %w", localCopyPath(), err)
	}
	basePath, err := os.MkdirTemp(localCopyPath(), prefix+".git")
	if err != nil {
		log.Error("Unable to create temporary directory: %s-*.git (%v)", prefix, err)
		return "", func() {}, fmt.Errorf("failed to create dir %s-*.git: %w", prefix, err)
	}
	return basePath, func() {
		if err := removeTemporaryPath(basePath); err != nil {
			log.Error("Unable to remove temporary directory: %s (%v)", basePath, err)
		}
	}, nil
}

// removeTemporaryPath removes the temporary path
func removeTemporaryPath(basePath string) error {
	if _, err := os.Stat(basePath); !os.IsNotExist(err) {
		return util.RemoveAll(basePath)
	}
	return nil
}
