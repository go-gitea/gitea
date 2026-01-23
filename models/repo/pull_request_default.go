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

// GetDefaultPRBaseBranchSetting returns the configured base branch for new pull requests.
// It returns an empty string when unset or pull requests are disabled.
func (repo *Repository) GetDefaultPRBaseBranchSetting(ctx context.Context) string {
	prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		return ""
	}
	cfg := prUnit.PullRequestsConfig()
	if cfg == nil {
		return ""
	}
	return cfg.DefaultBaseBranch
}

// GetDefaultPRBaseBranch returns the preferred base branch for new pull requests.
// It falls back to the repository default branch when unset or invalid.
func (repo *Repository) GetDefaultPRBaseBranch(ctx context.Context) string {
	preferred := repo.GetDefaultPRBaseBranchSetting(ctx)
	if preferred != "" {
		exists, err := isBranchNameExists(ctx, repo.ID, preferred)
		if err == nil && exists {
			return preferred
		}
	}
	return repo.DefaultBranch
}

// ValidateDefaultPRBaseBranch checks whether a preferred base branch is valid.
func (repo *Repository) ValidateDefaultPRBaseBranch(ctx context.Context, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}

	exists, err := isBranchNameExists(ctx, repo.ID, branch)
	if err != nil {
		return err
	}
	if !exists {
		return util.NewNotExistErrorf("default PR base branch does not exist [repo_id: %d name: %s]", repo.ID, branch)
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
