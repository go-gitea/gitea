// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

// DivergeObject represents commit count diverging commits
type DivergeObject struct {
	Ahead  int
	Behind int
}

// GetDivergingCommits returns the number of commits a targetBranch is ahead or behind a baseBranch
func GetDivergingCommits(ctx context.Context, repo RepositoryFacade, baseBranch, targetBranch string) (*DivergeObject, error) {
	cmd := gitcmd.NewCommand("rev-list", "--count", "--left-right").
		AddDynamicArguments(baseBranch + "..." + targetBranch).AddArguments("--")
	stdout, _, err1 := cmd.WithRepo(repo).RunStdString(ctx)
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

// GetCommitIDsBetweenReverse returns the last commit IDs between two commits in reverse order (from old to new) with limit.
// If the result exceeds the limit, the old commits IDs will be ignored
func GetCommitIDsBetweenReverse(ctx context.Context, repo RepositoryFacade, startRef, endRef, notRef string, limit int) ([]string, error) {
	cmd := gitcmd.NewCommand("rev-list", "--reverse").
		AddArguments("-n").AddDynamicArguments(strconv.Itoa(limit)).
		AddDynamicArguments(startRef + ".." + endRef)
	if notRef != "" { // --not should be kept as the last parameter of git command, otherwise the result will be wrong
		cmd.AddOptionValues("--not", notRef)
	}
	stdout, _, err := cmd.WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}

	commitIDs := strings.Fields(strings.TrimSpace(stdout))
	return commitIDs, nil
}
