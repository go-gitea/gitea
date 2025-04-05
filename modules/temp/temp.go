// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package temp

import (
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func MkdirTemp(pattern string) (string, func(), error) {
	dir, err := os.MkdirTemp(setting.TempPath, pattern)
	if err != nil {
		return "", nil, err
	}
	return dir, func() {
		if err := util.RemoveAll(dir); err != nil {
			log.Error("Failed to remove temp directory %s: %v", dir, err)
		}
	}, nil
}

func CreateTemp(pattern string) (*os.File, error) {
	return os.CreateTemp(setting.TempPath, pattern)
}
