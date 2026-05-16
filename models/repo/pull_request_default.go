// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"
)

func (repo *Repository) GetPullRequestTargetBranch(ctx context.Context) string {
	unitPRConfig := repo.MustGetUnit(ctx, unit.TypePullRequests).PullRequestsConfig()
	return util.IfZero(unitPRConfig.DefaultTargetBranch, repo.DefaultBranch)
}

// GetPullRequestDefaultBaseRepo returns the repository that should be used as
// the default base repository when creating a new pull request from repo.
func (repo *Repository) GetPullRequestDefaultBaseRepo(ctx context.Context) (*Repository, error) {
	if repo.IsFork {
		if err := repo.GetBaseRepo(ctx); err != nil {
			return nil, err
		}
		if repo.BaseRepo != nil && repo.BaseRepo.AllowsPulls(ctx) && repo.BaseRepo.CanContentChange() {
			return repo.BaseRepo, nil
		}
	}

	if repo.AllowsPulls(ctx) {
		return repo, nil
	}

	return nil, nil
}
