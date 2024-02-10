// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// CountDivergingCommits determines how many commits a branch is ahead or behind the repository's base branch
func CountDivergingCommits(ctx context.Context, repo Repository, baseBranch, branch string) (*git.DivergeObject, error) {
	var do git.DivergeObject
	cmd := git.NewCommand(ctx, "rev-list", "--count", "--left-right").
		AddDynamicArguments(baseBranch + "..." + branch)
	stdout, _, err := RunGitCmdStdString(repo, cmd, &RunOpts{})
	if err != nil {
		return &do, err
	}
	left, right, found := strings.Cut(strings.Trim(stdout, "\n"), "\t")
	if !found {
		return &do, fmt.Errorf("git rev-list output is missing a tab: %q", stdout)
	}

	behind, err1 := strconv.Atoi(left)
	if err1 != nil {
		return &do, err1
	}
	do.Behind = behind
	ahead, err1 := strconv.Atoi(right)
	if err1 != nil {
		return &do, err1
	}
	do.Ahead = ahead
	return &do, nil
}
