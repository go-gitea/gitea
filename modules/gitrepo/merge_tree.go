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
	switch len(fields) {
	case 0:
		return "", nil, errors.New("unexpected empty output")
	case 1:
		return strings.TrimSpace(fields[0]), nil, nil
	default:
		return strings.TrimSpace(fields[0]), fields[1:], nil
	}
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
	// For a successful, non-conflicted merge, the exit status is 0. When the merge has conflicts, the exit status is 1.
	// A merge can have conflicts without having individual files conflict
	// https://git-scm.com/docs/git-merge-tree/2.38.0#_mistakes_to_avoid
	switch {
	case gitcmd.IsErrorExitCode(gitErr, 0):
		return strings.TrimSpace(stdout.String()), false, nil, nil
	case gitcmd.IsErrorExitCode(gitErr, 1):
		treeID, conflictedFiles, err := parseMergeTreeOutput(stdout.String())
		if err != nil {
			return "", false, nil, fmt.Errorf("parse merge-tree output failed: %w", err)
		}
		return treeID, true, conflictedFiles, nil
	}
	return "", false, nil, fmt.Errorf("run merge-tree failed: %w", gitErr)
}
