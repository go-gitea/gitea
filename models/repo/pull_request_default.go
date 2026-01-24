// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"
)

// GetDefaultTargetBranchSetting returns the configured target branch for new pull requests.
// It returns an empty string when unset or pull requests are disabled.
func (repo *Repository) GetDefaultTargetBranchSetting(ctx context.Context) string {
	prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		return ""
	}
	cfg := prUnit.PullRequestsConfig()
	if cfg == nil {
		return ""
	}
	return cfg.DefaultTargetBranch
}

// GetDefaultTargetBranch returns the preferred target branch for new pull requests.
// It falls back to the repository default branch when unset or invalid.
func (repo *Repository) GetDefaultTargetBranch(ctx context.Context) string {
	preferred := repo.GetDefaultTargetBranchSetting(ctx)
	if preferred != "" {
		exists, err := isBranchNameExists(ctx, repo.ID, preferred)
		if err == nil && exists {
			return preferred
		}
	}
	return repo.DefaultBranch
}

// ValidateDefaultTargetBranch checks whether a preferred target branch is valid.
func (repo *Repository) ValidateDefaultTargetBranch(ctx context.Context, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}

	exists, err := isBranchNameExists(ctx, repo.ID, branch)
	if err != nil {
		return err
	}
	if !exists {
		return util.NewNotExistErrorf("default target branch does not exist [repo_id: %d name: %s]", repo.ID, branch)
	}
	return nil
}

func isBranchNameExists(ctx context.Context, repoID int64, branchName string) (bool, error) {
	type branch struct {
		IsDeleted bool `xorm:"is_deleted"`
	}
	var b branch
	has, err := db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		And("name = ?", branchName).
		Get(&b)
	if err != nil {
		return false, err
	}
	if !has {
		return false, nil
	}
	return !b.IsDeleted, nil
}
