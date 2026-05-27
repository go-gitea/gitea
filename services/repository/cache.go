// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
)

// CacheRef cachhe last commit information of the branch or the tag
func CacheRef(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, fullRefName git.RefName) error {
	commit, err := gitRepo.GetCommit(fullRefName.String())
	if err != nil {
		return err
	}

	if gitRepo.LastCommitCache == nil {
		commitsCount, err := cache.GetInt64(repo.GetCommitsCountCacheKey(fullRefName.ShortName(), true), func() (int64, error) {
			return gitrepo.CommitsCountOfCommit(ctx, repo, commit.ID.String())
		})
		if err != nil {
			return err
		}
		gitRepo.LastCommitCache = git.NewLastCommitCache(commitsCount, repo.FullName(), gitRepo, cache.GetCache())
	}

	return commit.CacheCommit(ctx)
}
