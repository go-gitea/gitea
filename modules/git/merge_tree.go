// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

// MergeTreeConflictStage is one side of a conflicted file in the index of a merge
type MergeTreeConflictStage struct {
	Mode     string
	ObjectID string
}

// MergeTreeConflict describes a conflicted path, a nil stage means the file doesn't exist on that side
type MergeTreeConflict struct {
	Path   string
	Base   *MergeTreeConflictStage // the merge base
	Ours   *MergeTreeConflictStage // baseRef
	Theirs *MergeTreeConflictStage // headRef
}

// MergeTreeConflicts merges headRef into baseRef like MergeTree, but returns details for conflicted files (at most limit).
// hasConflicts can be true without any listed file.
// treeID is the tree of the merge result, in which conflicted files contain conflict markers.
func MergeTreeConflicts(ctx context.Context, repo RepositoryFacade, baseRef, headRef, mergeBase string, limit int) (treeID string, hasConflicts bool, conflicts []*MergeTreeConflict, truncated bool, _ error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--no-messages").WithRepo(repo)
	if mergeBase != "" {
		cmd.AddOptionFormat("--merge-base=%s", mergeBase)
	}
	cmd.AddDynamicArguments(baseRef, headRef)

	byPath := map[string]*MergeTreeConflict{}
	err := gitcmd.RunWithScannerSplit(ctx, cmd, util.BufioScannerSplit(0), func(ctx gitcmd.Context, chunk []byte) error {
		// https://git-scm.com/docs/git-merge-tree#_output
		// <OID of toplevel tree>NUL
		// <Conflicted file info>NUL, in the format "<mode> <object> <stage><TAB><filename>"
		if treeID == "" {
			treeID = string(chunk)
			return nil
		}
		path, stage, ok := parseMergeTreeConflictInfo(string(chunk))
		if !ok {
			return nil
		}
		conflict := byPath[path.path]
		if conflict == nil {
			if len(conflicts) >= limit {
				truncated = true
				return ctx.CancelPipeline(nil)
			}
			conflict = &MergeTreeConflict{Path: path.path}
			byPath[path.path] = conflict
			conflicts = append(conflicts, conflict)
		}
		switch stage {
		case 1:
			conflict.Base = &path.stage
		case 2:
			conflict.Ours = &path.stage
		case 3:
			conflict.Theirs = &path.stage
		}
		return nil
	})
	// a conflicted merge can have no conflicted file entries, then only the exit code tells
	hasConflicts = len(conflicts) > 0 || gitcmd.IsErrorExitCode(err, 1)
	if err == nil || truncated || hasConflicts {
		return treeID, hasConflicts, conflicts, truncated, nil
	}
	return "", false, nil, false, fmt.Errorf("run merge-tree failed: %w", err)
}

type mergeTreeConflictInfo struct {
	path  string
	stage MergeTreeConflictStage
}

// parseMergeTreeConflictInfo parses "<mode> <object> <stage><TAB><filename>"
func parseMergeTreeConflictInfo(line string) (info mergeTreeConflictInfo, stage int, ok bool) {
	meta, path, found := strings.Cut(line, "\t")
	if !found || path == "" {
		return info, 0, false
	}
	fields := strings.Fields(meta)
	if len(fields) != 3 {
		return info, 0, false
	}
	stage, err := strconv.Atoi(fields[2])
	if err != nil || stage < 1 || stage > 3 {
		return info, 0, false
	}
	return mergeTreeConflictInfo{path: path, stage: MergeTreeConflictStage{Mode: fields[0], ObjectID: fields[1]}}, stage, true
}
