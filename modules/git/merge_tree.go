// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

const maxConflictedDetectFiles = 10

// MergeTree performs a merge between two commits (baseRef and headRef) with an optional merge base.
// It returns the resulting tree hash, a list of conflicted files (if any), and an error if the operation fails.
// If there are no conflicts, the list of conflicted files will be nil.
func MergeTree(ctx context.Context, repo RepositoryFacade, baseRef, headRef, mergeBase string) (treeID string, hasConflicts bool, conflictFiles []string, _ error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages").WithRepo(repo)
	if mergeBase != "" {
		cmd.AddOptionFormat("--merge-base=%s", mergeBase)
	}
	cmd.AddDynamicArguments(baseRef, headRef)

	err := gitcmd.RunWithScannerSplit(ctx, cmd, util.BufioScannerSplit(0), func(ctx gitcmd.Context, chunk []byte) error {
		// https://git-scm.com/docs/git-merge-tree/2.38.0#OUTPUT
		// For a conflicted merge, the output is:
		// <OID of toplevel tree>NUL
		// <Conflicted file name 1>NUL
		// <Conflicted file name 2>NUL
		// ...
		line := string(chunk)
		if treeID == "" { // first line is tree ID
			treeID = line
		} else if len(conflictFiles) < maxConflictedDetectFiles {
			conflictFiles = append(conflictFiles, line)
		} else {
			return ctx.CancelPipeline(nil)
		}
		return nil
	})

	// For a successful, non-conflicted merge, the exit status is 0. When the merge has conflicts, the exit status is 1.
	// A merge can have conflicts without having individual files conflict
	// https://git-scm.com/docs/git-merge-tree/2.38.0#_mistakes_to_avoid
	hasConflicts = len(conflictFiles) > 0 || gitcmd.IsErrorExitCode(err, 1)
	if err == nil || hasConflicts {
		return treeID, hasConflicts, conflictFiles, nil
	}
	return "", false, nil, fmt.Errorf("run merge-tree failed: %w", err)
}
