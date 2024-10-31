// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/setting"
)

var curService Service

func Init(ctx context.Context) error {
	curService = &localServiceImpl{
		repoRootDir: setting.RepoRootPath,
	}
	return nil
}
