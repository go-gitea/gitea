// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// DivergeObject represents commit count diverging commits
type DivergeObject struct {
	Ahead  int
	Behind int
}

// GetDivergingCommits returns the number of commits a targetBranch is ahead or behind a baseBranch
func GetDivergingCommits(ctx context.Context, repo Repository, baseBranch, targetBranch string) (*DivergeObject, error) {
	cmd := gitcmd.NewCommand("rev-list", "--count", "--left-right").
		AddDynamicArguments(baseBranch + "..." + targetBranch).AddArguments("--")
	stdout, _, err1 := RunCmdString(ctx, repo, cmd)
	if err1 != nil {
		return nil, err1
	}

	left, right, found := strings.Cut(strings.Trim(stdout, "\n"), "\t")
	if !found {
		return nil, fmt.Errorf("git rev-list output is missing a tab: %q", stdout)
	}

	behind, err := strconv.Atoi(left)
	if err != nil {
		return nil, err
	}
	ahead, err := strconv.Atoi(right)
	if err != nil {
		return nil, err
	}
	return &DivergeObject{Ahead: ahead, Behind: behind}, nil
}

// GetCommitIDsBetweenReversed returns the commit IDs between two commits in reverse order(from old to new)
func GetCommitIDsBetweenReversed(ctx context.Context, repo Repository, startRef, endRef string) ([]string, error) {
	stdout, _, err := RunCmdString(ctx, repo, gitcmd.NewCommand("rev-list", "--reverse").AddDynamicArguments(startRef+".."+endRef))
	// example git error message: fatal: origin/main...HEAD: no merge base
	if err != nil && strings.Contains(err.Stderr(), "no merge base") {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		stdout, _, err = RunCmdString(ctx, repo, gitcmd.NewCommand("rev-list", "--reverse").AddDynamicArguments(startRef, endRef))
	}
	if err != nil {
		return nil, err
	}

	commitIDs := strings.Fields(strings.TrimSpace(stdout))
	return commitIDs, nil
}
