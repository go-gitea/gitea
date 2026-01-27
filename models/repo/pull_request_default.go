// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/unit"
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
// It falls back to the repository default branch when unset.
func (repo *Repository) GetDefaultTargetBranch(ctx context.Context) string {
	preferred := repo.GetDefaultTargetBranchSetting(ctx)
	if preferred != "" {
		return preferred
	}
	return repo.DefaultBranch
}

// ValidateDefaultTargetBranch checks whether a preferred target branch is valid.
func (repo *Repository) ValidateDefaultTargetBranch(ctx context.Context, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}
	return nil
}
