// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git/gitcmd"
)

func LocalCodeGitRepo(ownerName, repoName string) gitcmd.RepositoryFacade {
	return repo_model.CodeRepoByName(ownerName, repoName)
}
