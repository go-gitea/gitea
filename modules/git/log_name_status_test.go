// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildManyCommitsRepo creates a repository with enough commits that
// "git log --name-status" produces more output than the OS pipe buffer (64KB),
// so the git command stays blocked on write while the caller is still reading.
func buildManyCommitsRepo(t *testing.T, commits, filesPerCommit int) string {
	t.Helper()
	dir := t.TempDir()

	initCmd := exec.Command("git", "init", "-q", "-b", "main")
	initCmd.Dir = dir
	require.NoError(t, initCmd.Run())

	// Build a fast-import stream so the whole history is created by a single
	// git process, which keeps the test fast (sub-second).
	var stream strings.Builder
	mark := 0
	for c := range commits {
		blobMarks := make([]int, filesPerCommit)
		for f := range filesPerCommit {
			mark++
			blobMarks[f] = mark
			content := fmt.Sprintf("commit %d file %d\n", c, f)
			fmt.Fprintf(&stream, "blob\nmark :%d\ndata %d\n%s\n", mark, len(content), content)
		}
		mark++
		msg := fmt.Sprintf("commit %d", c)
		// fast-import implicitly uses the branch tip as the parent, so no "from" is needed.
		fmt.Fprintf(&stream, "commit refs/heads/main\nmark :%d\n", mark)
		fmt.Fprintf(&stream, "committer a <a@example.com> %d +0000\n", 1700000000+c)
		fmt.Fprintf(&stream, "data %d\n%s\n", len(msg), msg)
		for f := range filesPerCommit {
			fmt.Fprintf(&stream, "M 644 :%d file%d.txt\n", blobMarks[f], f)
		}
		stream.WriteString("\n")
	}

	importCmd := exec.Command("git", "fast-import", "--quiet")
	importCmd.Dir = dir
	importCmd.Stdin = strings.NewReader(stream.String())
	importCmd.Stderr = os.Stderr
	require.NoError(t, importCmd.Run())
	return dir
}

// TestWalkGitLogTimeoutReturnsPartialResults is a regression test for a 500 error
// ("read |0: file already closed") on the repository home page of large repos.
//
// The directory listing computes the latest commit per entry under a short timeout
// and falls back to async loading when it expires. When the timeout fires, the git
// command is killed and its output pipe is closed, so the in-flight read fails with
// an os.ErrClosed-style error instead of context.DeadlineExceeded. WalkGitLog must
// recognise the cancelled context and return the partial results gathered so far
// rather than propagating the read error (which became a 500).
func TestWalkGitLogTimeoutReturnsPartialResults(t *testing.T) {
	dir := buildManyCommitsRepo(t, 1500, 5)
	repo, err := OpenRepository(t.Context(), dir)
	require.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetBranchCommit("main")
	require.NoError(t, err)
	entries, err := commit.Tree.ListEntries()
	require.NoError(t, err)

	// Several iterations to make sure the deadline reliably fires while the walk is
	// still reading from the (blocked) git command. Without the fix this returns an
	// error such as "read |0: file already closed" on the very first iteration.
	for i := range 20 {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
		_, _, err := entries.GetCommitsInfo(ctx, "/any/repo-link", commit, "")
		cancel()
		require.NoError(t, err, "GetCommitsInfo must not error when the commit-info timeout expires (iteration %d)", i)
	}
}
