// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

type Repository = git.Repository

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repoPath *repo_model.Repository) (*Repository, error) {
	return git.OpenRepository(ctx, repoPath.RepoPath())
}

// DeleteRepository deletes the repository at the given relative path with the provided context.
func DeleteRepository(ctx context.Context, repo *repo_model.Repository) error {
	if err := util.RemoveAll(repo.RepoPath()); err != nil {
		return fmt.Errorf("Failed to remove %s: %w", repo.FullName(), err)
	}
	return nil
}
