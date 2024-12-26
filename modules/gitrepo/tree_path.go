// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

func GetTreePathLatestCommit(ctx context.Context, repo Repository, refName, treePath string) (*git.Commit, error) {
	gitRepo, closer, err := RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	latestCommitID, err := gitRepo.GetTreePathLatestCommitID(refName, treePath)
	if err != nil {
		return nil, err
	}

	return gitRepo.GetCommit(latestCommitID)
}
