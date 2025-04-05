// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// localCopyPath returns the local repository temporary copy path.
func localCopyPath() string {
	return filepath.Join(setting.TempPath, "local-repo")
}

// CreateTemporaryPath creates a temporary path
func CreateTemporaryPath(prefix string) (string, context.CancelFunc, error) {
	if err := os.MkdirAll(localCopyPath(), os.ModePerm); err != nil {
		log.Error("Unable to create localcopypath directory: %s (%v)", localCopyPath(), err)
		return "", nil, fmt.Errorf("failed to create localcopypath directory %s: %w", localCopyPath(), err)
	}
	basePath, err := os.MkdirTemp(localCopyPath(), prefix+".git")
	if err != nil {
		log.Error("Unable to create temporary directory: %s-*.git (%v)", prefix, err)
		return "", nil, fmt.Errorf("failed to create dir %s-*.git: %w", prefix, err)
	}
	return basePath, func() {
		if err := util.RemoveAll(basePath); err != nil {
			log.Error("Unable to remove temporary directory: %s (%v)", basePath, err)
		}
	}, nil
}
