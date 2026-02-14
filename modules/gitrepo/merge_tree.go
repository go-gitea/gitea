// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
)

const MaxConflictedDetectFiles = 10

// MergeTree performs a merge between two commits (baseRef and headRef) with an optional merge base.
// It returns the resulting tree hash, a list of conflicted files (if any), and an error if the operation fails.
// If there are no conflicts, the list of conflicted files will be nil.
func MergeTree(ctx context.Context, repo Repository, baseRef, headRef, mergeBase string) (treeID string, isErrHasConflicts bool, conflictFiles []string, _ error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages").
		AddOptionFormat("--merge-base=%s", mergeBase).
		AddDynamicArguments(baseRef, headRef)

	stdout, stdoutClose := cmd.MakeStdoutPipe()
	defer stdoutClose()
	cmd.WithPipelineFunc(func(ctx gitcmd.Context) error {
		// https://git-scm.com/docs/git-merge-tree/2.38.0#OUTPUT
		// For a conflicted merge, the output is:
		// <OID of toplevel tree>NUL
		// <Conflicted file name 1>NUL
		// <Conflicted file name 2>NUL
		// ...
		scanner := bufio.NewScanner(stdout)
		scanner.Split(util.BufioScannerSplit(0))
		for scanner.Scan() {
			line := scanner.Text()
			if treeID == "" { // first line is tree ID
				treeID = line
				continue
			}
			conflictFiles = append(conflictFiles, line)
			if len(conflictFiles) >= MaxConflictedDetectFiles {
				break
			}
		}
		return scanner.Err()
	})

	err := RunCmdWithStderr(ctx, repo, cmd)
	// For a successful, non-conflicted merge, the exit status is 0. When the merge has conflicts, the exit status is 1.
	// A merge can have conflicts without having individual files conflict
	// https://git-scm.com/docs/git-merge-tree/2.38.0#_mistakes_to_avoid
	isErrHasConflicts = gitcmd.IsErrorExitCode(err, 1)
	if err == nil || isErrHasConflicts {
		return treeID, isErrHasConflicts, conflictFiles, nil
	}
	return "", false, nil, fmt.Errorf("run merge-tree failed: %w", err)
}
