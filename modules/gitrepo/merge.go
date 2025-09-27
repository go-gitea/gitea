// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

func MergeBase(ctx context.Context, repo Repository, commit1, commit2 string) (string, error) {
	mergeBase, _, err := gitcmd.NewCommand("merge-base", "--").
		AddDynamicArguments(commit1, commit2).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	if err != nil {
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", commit1, commit2, err)
	}
	return strings.TrimSpace(mergeBase), nil
}

func MergeTree(ctx context.Context, repo Repository, base, ours, theirs string) (string, bool, []string, error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages")
	// https://git-scm.com/docs/git-merge-tree/2.40.0#_mistakes_to_avoid
	if git.DefaultFeatures().CheckVersionAtLeast("2.40") && !git.DefaultFeatures().CheckVersionAtLeast("2.41") {
		cmd.AddOptionFormat("--merge-base=%s", base)
	}

	stdout := &bytes.Buffer{}
	gitErr := cmd.AddDynamicArguments(ours, theirs).Run(ctx, &gitcmd.RunOpts{
		Dir:    repoPath(repo),
		Stdout: stdout,
	})
	if gitErr != nil && !gitcmd.IsErrorExitCode(gitErr, 1) {
		log.Error("run merge-tree failed: %v", gitErr)
		return "", false, nil, fmt.Errorf("run merge-tree failed: %w", gitErr)
	}

	// There are two situations that we consider for the output:
	// 1. Clean merge and the output is <OID of toplevel tree>NUL
	// 2. Merge conflict and the output is <OID of toplevel tree>NUL<Conflicted file info>NUL
	treeOID, conflictedFileInfo, _ := strings.Cut(stdout.String(), "\x00")
	if len(conflictedFileInfo) == 0 {
		return treeOID, gitcmd.IsErrorExitCode(gitErr, 1), nil, nil
	}

	// Remove last NULL-byte from conflicted file info, then split with NULL byte as separator.
	return treeOID, true, strings.Split(conflictedFileInfo[:len(conflictedFileInfo)-1], "\x00"), nil
}
