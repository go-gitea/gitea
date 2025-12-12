// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CompareInfo represents the collected results from ParseCompareInfo
type CompareInfo struct {
	HeadUser         *user_model.User
	HeadRepo         *repo_model.Repository
	HeadGitRepo      *git.Repository
	CompareInfo      *pull_service.CompareInfo
	BaseBranch       string
	HeadBranch       string
	DirectComparison bool
}

// maxForkTraverseLevel defines the maximum levels to traverse when searching for the head repository.
const maxForkTraverseLevel = 10

// FindHeadRepo tries to find the head repository based on the base repository and head user ID.
func FindHeadRepo(ctx context.Context, baseRepo *repo_model.Repository, headUserID int64) (*repo_model.Repository, error) {
	if baseRepo.IsFork {
		curRepo := baseRepo
		for curRepo.OwnerID != headUserID { // We assume the fork deepth is not too deep.
			if err := curRepo.GetBaseRepo(ctx); err != nil {
				return nil, err
			}
			if curRepo.BaseRepo == nil {
				return findHeadRepoFromRootBase(ctx, curRepo, headUserID, maxForkTraverseLevel)
			}
			curRepo = curRepo.BaseRepo
		}
		return curRepo, nil
	}

	return findHeadRepoFromRootBase(ctx, baseRepo, headUserID, maxForkTraverseLevel)
}

func findHeadRepoFromRootBase(ctx context.Context, baseRepo *repo_model.Repository, headUserID int64, traverseLevel int) (*repo_model.Repository, error) {
	if traverseLevel == 0 {
		return nil, nil
	}
	// test if we are lucky
	repo, err := repo_model.GetUserFork(ctx, baseRepo.ID, headUserID)
	if err != nil {
		return nil, err
	}
	if repo != nil {
		return repo, nil
	}

	firstLevelForkedRepos, err := repo_model.GetRepositoriesByForkID(ctx, baseRepo.ID)
	if err != nil {
		return nil, err
	}
	for _, repo := range firstLevelForkedRepos {
		forked, err := findHeadRepoFromRootBase(ctx, repo, headUserID, traverseLevel-1)
		if err != nil {
			return nil, err
		}
		if forked != nil {
			return forked, nil
		}
	}
	return nil, nil
}
