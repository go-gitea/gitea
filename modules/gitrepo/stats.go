// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(ctx context.Context, repo Repository, baseBranch, branch string) (*git.DivergeObject, error) {
	divergence, err := git.GetDivergingCommits(ctx, repoPath(repo), baseBranch, branch)
	if err != nil {
		return nil, err
	}
	return &divergence, nil
}
