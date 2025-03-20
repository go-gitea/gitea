// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
)

func GetRefCommitsCount(ctx context.Context, repoID int64, refFullName git.RefName) (int64, error) {
	// Get the commit count of the branch or the tag
	switch {
	case refFullName.IsBranch():
		branch, err := git_model.GetBranch(ctx, repoID, refFullName.BranchName())
		if err != nil {
			return 0, err
		}
		return branch.CommitCount, nil
	case refFullName.IsTag():
		tag, err := repo_model.GetRelease(ctx, repoID, refFullName.TagName())
		if err != nil {
			return 0, err
		}
		return tag.NumCommits, nil
	default:
		return 0, nil
	}
}

// CacheRef cachhe last commit information of the branch or the tag
func CacheRef(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, fullRefName git.RefName) error {
	commit, err := gitRepo.GetCommit(fullRefName.String())
	if err != nil {
		return err
	}

	if gitRepo.LastCommitCache == nil {
		commitsCount, err := GetRefCommitsCount(ctx, repo.ID, fullRefName)
		if err != nil {
			return err
		}
		gitRepo.LastCommitCache = git.NewLastCommitCache(commitsCount, repo.FullName(), gitRepo, cache.GetCache())
	}

	return commit.CacheCommit(ctx)
}
