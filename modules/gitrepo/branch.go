// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

// GetBranchesByPath returns a branch by it's path
// if limit = 0 it will not limit
func GetBranchesByPath(ctx context.Context, repo *repo_model.Repository, skip, limit int) ([]*git.Branch, int, error) {
	gitRepo, err := OpenRepository(ctx, repo)
	if err != nil {
		return nil, 0, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranches(skip, limit)
}

func GetBranchCommitID(ctx context.Context, repo *repo_model.Repository, branch string) (string, error) {
	gitRepo, err := OpenRepository(ctx, repo)
	if err != nil {
		return "", err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranchCommitID(branch)
}
