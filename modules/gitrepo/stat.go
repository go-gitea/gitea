// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"os"

	"code.gitea.io/gitea/modules/setting"
)

func RepositoryStoreStat() error {
	_, err := os.Stat(setting.RepoRootPath)
	return err
}
