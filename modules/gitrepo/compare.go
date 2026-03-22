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

// GetCommitIDsBetween returns the commit IDs between startRef and endRef, if notRef is not empty,
// it will exclude commits reachable from notRef.
func GetCommitIDsBetween(ctx context.Context, repo Repository, startRef, endRef, notRef string) ([]string, error) {
	genBaseCmd := func() *gitcmd.Command {
		baseCmd := gitcmd.NewCommand("rev-list", "--reverse")
		if notRef != "" {
			baseCmd.AddOptionValues("--not", notRef)
		}
		return baseCmd
	}

	stdout, _, err := RunCmdString(ctx, repo, genBaseCmd().AddDynamicArguments(startRef+".."+endRef))
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		stdout, _, err = RunCmdString(ctx, repo, genBaseCmd().AddDynamicArguments(startRef, endRef))
	}
	if err != nil {
		return nil, err
	}

	commitIDs := strings.Fields(strings.TrimSpace(stdout))
	return commitIDs, nil
}
