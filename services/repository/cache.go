// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

// CacheRef cachhe last commit information of the branch or the tag
func CacheRef(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, fullRefName git.RefName) error {
	commit, err := gitRepo.GetCommit(fullRefName.String())
	if err != nil {
		return err
	}

	return gitRepo.CacheCommit(ctx, commit)
}
