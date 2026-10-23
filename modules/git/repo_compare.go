// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"context"
	"io"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

type lineCountWriter struct {
	numLines int
}

// Write counts the number of newlines in the provided bytestream
func (l *lineCountWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	l.numLines += bytes.Count(p, []byte{'\000'})
	return n, err
}

// GetDiffNumChangedFiles counts the number of changed files
// This is substantially quicker than shortstat but...
func (repo *Repository) GetDiffNumChangedFiles(ctx context.Context, base, head string, directComparison bool) (int, error) {
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	w := &lineCountWriter{}

	separator := "..."
	if directComparison {
		separator = ".."
	}

	if err := gitcmd.NewCommand("diff", "-z", "--name-only").
		AddDynamicArguments(base + separator + head).
		AddArguments("--").
		WithRepo(repo).
		WithStdoutCopy(w).
		RunWithStderr(ctx); err != nil {
		return 0, err
	}
	return w.numLines, nil
}

// GetDiff generates and returns patch data between given revisions, optimized for human readability
func (repo *Repository) GetDiff(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p").AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		Run(ctx)
}

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func (repo *Repository) GetDiffBinary(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p", "--binary", "--histogram").
		AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		Run(ctx)
}

// GetPatch generates and returns format-patch data between given revisions, able to be used with `git apply`
func (repo *Repository) GetPatch(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("format-patch", "--binary", "--stdout").AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		Run(ctx)
}

// GetFilesChangedBetween returns a list of all files that have been changed between the given commits
// If base is undefined empty SHA (zeros), it only returns the files changed in the head commit
// If base is the SHA of an empty tree (EmptyTreeSHA), it returns the files changes from the initial commit to the head commit
func (repo *Repository) GetFilesChangedBetween(ctx context.Context, base, head string) ([]string, error) {
	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return nil, err
	}
	cmd := gitcmd.NewCommand("diff-tree", "--name-only", "--root", "--no-commit-id", "-r", "-z")
	if base == objectFormat.EmptyObjectID().String() {
		cmd.AddDynamicArguments(head)
	} else {
		cmd.AddDynamicArguments(base, head)
	}
	stdout, _, err := cmd.WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	split := strings.Split(stdout, "\000")

	// Because Git will always emit filenames with a terminal NUL ignore the last entry in the split - which will always be empty.
	if len(split) > 0 {
		split = split[:len(split)-1]
	}

	return split, err
}
