// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/objectpool"
)

func GetObjectPoolProvider(repo Repository) objectpool.Provider {
	return git.NewObjectPoolProvider(repoPath(repo))
}
