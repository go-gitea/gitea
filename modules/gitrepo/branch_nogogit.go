// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetBranchNames returns branches from the repository, skipping "skip" initial branches and
// returning at most "limit" branches, or all branches if "limit" is 0.
func GetBranchNames(ctx context.Context, repo Repository, skip, limit int) ([]string, int, error) {
	return callShowRef(ctx, repo, git.BranchPrefix, gitcmd.TrustedCmdArgs{git.BranchPrefix, "--sort=-committerdate"}, skip, limit)
}
