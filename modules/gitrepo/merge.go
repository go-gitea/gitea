// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

func MergeBase(ctx context.Context, repo Repository, commit1, commit2 string) (string, error) {
	mergeBase, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base", "--").
		AddDynamicArguments(commit1, commit2))
	if err != nil {
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", commit1, commit2, err)
	}
	return strings.TrimSpace(mergeBase), nil
}

// parseMergeTreeOutput parses the output of git merge-tree --write-tree -z --name-only --no-messages
// For a successful merge, the output is a simply one line <OID of toplevel tree>NUL
// Whereas for a conflicted merge, the output is:
// <OID of toplevel tree>NUL
// <Conflicted file name 1>NUL
// <Conflicted file name 2>NUL
// ...
// ref: https://git-scm.com/docs/git-merge-tree/2.38.0#OUTPUT
func parseMergeTreeOutput(output string) (string, []string, error) {
	fields := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	if len(fields) == 0 {
		return "", nil, errors.New("unexpected empty output")
	}
	if len(fields) == 1 {
		return strings.TrimSpace(fields[0]), nil, nil
	}
	// ignore the last one because it's always empty after the last NUL
	return strings.TrimSpace(fields[0]), fields[1:], nil
}

// MergeTree performs a merge between two commits (baseRef and headRef) with an optional merge base.
// It returns the resulting tree hash, a list of conflicted files (if any), and an error if the operation fails.
// If there are no conflicts, the list of conflicted files will be nil.
func MergeTree(ctx context.Context, repo Repository, baseRef, headRef, mergeBase string) (string, bool, []string, error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages")
	if git.DefaultFeatures().CheckVersionAtLeast("2.40") && mergeBase != "" {
		cmd.AddOptionFormat("--merge-base=%s", mergeBase)
	}

	stdout := &bytes.Buffer{}
	gitErr := RunCmd(ctx, repo, cmd.AddDynamicArguments(baseRef, headRef).WithStdout(stdout))
	exitCode, ok := gitcmd.ExitCode(gitErr)
	if !ok {
		return "", false, nil, fmt.Errorf("run merge-tree failed: %w", gitErr)
	}

	switch exitCode {
	case 0, 1:
		treeID, conflictedFiles, err := parseMergeTreeOutput(stdout.String())
		if err != nil {
			return "", false, nil, fmt.Errorf("parse merge-tree output failed: %w", err)
		}
		// For a successful, non-conflicted merge, the exit status is 0. When the merge has conflicts, the exit status is 1.
		// A merge can have conflicts without having individual files conflict
		// https://git-scm.com/docs/git-merge-tree/2.38.0#_mistakes_to_avoid
		return treeID, exitCode == 1, conflictedFiles, nil
	default:
		return "", false, nil, fmt.Errorf("run merge-tree exit abnormally: %w", gitErr)
	}
}
