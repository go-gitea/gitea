// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetMergeBase checks and returns merge base of two branches and the reference used as base.
func (repo *Repository) GetMergeBase(tmpRemote, base, head string) (string, string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := RemotePrefix + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, _, err := gitcmd.NewCommand("fetch", "--no-tags").
			AddDynamicArguments(tmpRemote).
			AddDashesAndList(base + ":" + tmpBaseName).
			WithDir(repo.Path).
			RunStdString(repo.Ctx)
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, _, err := gitcmd.NewCommand("merge-base").
		AddDashesAndList(base, head).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	return strings.TrimSpace(stdout), base, err
}

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
func (repo *Repository) GetDiffNumChangedFiles(base, head string, directComparison bool) (int, error) {
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	w := &lineCountWriter{}
	stderr := new(bytes.Buffer)

	separator := "..."
	if directComparison {
		separator = ".."
	}

	// avoid: ambiguous argument 'refs/a...refs/b': unknown revision or path not in the working tree. Use '--': 'git <command> [<revision>...] -- [<file>...]'
	if err := gitcmd.NewCommand("diff", "-z", "--name-only").
		AddDynamicArguments(base + separator + head).
		AddArguments("--").
		WithDir(repo.Path).
		WithStdout(w).
		WithStderr(stderr).
		Run(repo.Ctx); err != nil {
		if strings.Contains(stderr.String(), "no merge base") {
			// git >= 2.28 now returns an error if base and head have become unrelated.
			// previously it would return the results of git diff -z --name-only base head so let's try that...
			w = &lineCountWriter{}
			stderr.Reset()
			if err = gitcmd.NewCommand("diff", "-z", "--name-only").
				AddDynamicArguments(base, head).
				AddArguments("--").
				WithDir(repo.Path).
				WithStdout(w).
				WithStderr(stderr).
				Run(repo.Ctx); err == nil {
				return w.numLines, nil
			}
		}
		return 0, fmt.Errorf("%w: Stderr: %s", err, stderr)
	}
	return w.numLines, nil
}

var patchCommits = regexp.MustCompile(`^From\s(\w+)\s`)

// GetDiff generates and returns patch data between given revisions, optimized for human readability
func (repo *Repository) GetDiff(compareArg string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	return gitcmd.NewCommand("diff", "-p").AddDynamicArguments(compareArg).
		WithDir(repo.Path).
		WithStdout(w).
		WithStderr(stderr).
		Run(repo.Ctx)
}

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func (repo *Repository) GetDiffBinary(compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p", "--binary", "--histogram").
		AddDynamicArguments(compareArg).
		WithDir(repo.Path).
		WithStdout(w).
		Run(repo.Ctx)
}

// GetPatch generates and returns format-patch data between given revisions, able to be used with `git apply`
func (repo *Repository) GetPatch(compareArg string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	return gitcmd.NewCommand("format-patch", "--binary", "--stdout").AddDynamicArguments(compareArg).
		WithDir(repo.Path).
		WithStdout(w).
		WithStderr(stderr).
		Run(repo.Ctx)
}

// GetFilesChangedBetween returns a list of all files that have been changed between the given commits
// If base is undefined empty SHA (zeros), it only returns the files changed in the head commit
// If base is the SHA of an empty tree (EmptyTreeSHA), it returns the files changes from the initial commit to the head commit
func (repo *Repository) GetFilesChangedBetween(base, head string) ([]string, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	cmd := gitcmd.NewCommand("diff-tree", "--name-only", "--root", "--no-commit-id", "-r", "-z")
	if base == objectFormat.EmptyObjectID().String() {
		cmd.AddDynamicArguments(head)
	} else {
		cmd.AddDynamicArguments(base, head)
	}
	stdout, _, err := cmd.WithDir(repo.Path).RunStdString(repo.Ctx)
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

// ReadPatchCommit will check if a diff patch exists and return stats
func (repo *Repository) ReadPatchCommit(prID int64) (commitSHA string, err error) {
	// Migrated repositories download patches to "pulls" location
	patchFile := fmt.Sprintf("pulls/%d.patch", prID)
	loadPatch, err := os.Open(filepath.Join(repo.Path, patchFile))
	if err != nil {
		return "", err
	}
	defer loadPatch.Close()
	// Read only the first line of the patch - usually it contains the first commit made in patch
	scanner := bufio.NewScanner(loadPatch)
	scanner.Scan()
	// Parse the Patch stats, sometimes Migration returns a 404 for the patch file
	commitSHAGroups := patchCommits.FindStringSubmatch(scanner.Text())
	if len(commitSHAGroups) != 0 {
		commitSHA = commitSHAGroups[1]
	} else {
		return "", errors.New("patch file doesn't contain valid commit ID")
	}
	return commitSHA, nil
}
