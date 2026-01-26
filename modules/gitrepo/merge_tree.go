// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

func ScanNullTerminatedStrings(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseMergeTreeOutput parses the output of git merge-tree --write-tree -z --name-only --no-messages
// For a successful merge, the output is a simply one line <OID of toplevel tree>NUL
// Whereas for a conflicted merge, the output is:
// <OID of toplevel tree>NUL
// <Conflicted file name 1>NUL
// <Conflicted file name 2>NUL
// ...
// ref: https://git-scm.com/docs/git-merge-tree/2.38.0#OUTPUT
func parseMergeTreeOutput(output io.Reader, maxListFiles int) (treeID string, conflictedFiles []string, err error) {
	scanner := bufio.NewScanner(output)

	scanner.Split(ScanNullTerminatedStrings)
	var lineCount int
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if treeID == "" { // first line is tree ID
			treeID = line
			continue
		}
		conflictedFiles = append(conflictedFiles, line)
		lineCount++
		if lineCount >= maxListFiles {
			break
		}
	}
	if treeID == "" {
		return "", nil, errors.New("unexpected empty output")
	}
	return treeID, conflictedFiles, scanner.Err()
}

const MaxConflictedDetectFiles = 10

// MergeTree performs a merge between two commits (baseRef and headRef) with an optional merge base.
// It returns the resulting tree hash, a list of conflicted files (if any), and an error if the operation fails.
// If there are no conflicts, the list of conflicted files will be nil.
func MergeTree(ctx context.Context, repo Repository, baseRef, headRef, mergeBase string) (string, bool, []string, error) {
	cmd := gitcmd.NewCommand("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages")
	if git.DefaultFeatures().CheckVersionAtLeast("2.40") && mergeBase != "" {
		cmd.AddOptionFormat("--merge-base=%s", mergeBase)
	}

	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	gitErr := RunCmd(ctx, repo, cmd.AddDynamicArguments(baseRef, headRef))
	// For a successful, non-conflicted merge, the exit status is 0. When the merge has conflicts, the exit status is 1.
	// A merge can have conflicts without having individual files conflict
	// https://git-scm.com/docs/git-merge-tree/2.38.0#_mistakes_to_avoid
	switch {
	case gitErr == nil:
		bs, err := io.ReadAll(stdoutReader)
		if err != nil {
			return "", false, nil, fmt.Errorf("read merge-tree output failed: %w", err)
		}
		return strings.TrimSpace(strings.TrimSuffix(string(bs), "\x00")), false, nil, nil
	case gitcmd.IsErrorExitCode(gitErr, 1):
		treeID, conflictedFiles, err := parseMergeTreeOutput(stdoutReader, MaxConflictedDetectFiles)
		if err != nil {
			return "", false, nil, fmt.Errorf("parse merge-tree output failed: %w", err)
		}
		return treeID, true, conflictedFiles, nil
	}
	return "", false, nil, fmt.Errorf("run merge-tree failed: %w", gitErr)
}
