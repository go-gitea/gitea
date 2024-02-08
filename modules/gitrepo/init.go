// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

var curService Service

func Init(ctx context.Context) error {
	curService = &localServiceImpl{
		repoRootDir: setting.RepoRootPath,
	}
	return nil
}

// FIXME:
func VersionInfo() string {
	return git.VersionInfo()
}

// FIXME:
func HomeDir() string {
	return setting.Git.HomePath
}
