// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// GetDiffNumChangedFiles counts the number of changed files
// This is substantially quicker than shortstat but...
func (repo *Repository) GetDiffNumChangedFiles(ctx context.Context, base, head string, directComparison bool) (int, error) {
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly

	separator := "..."
	if directComparison {
		separator = ".."
	}

	cmd := gitcmd.NewCommand("diff", "-z", "--name-only").
		AddDynamicArguments(base + separator + head).
		AddArguments("--").
		WithRepo(repo)

	count := 0
	err := gitcmd.RunWithScannerSplit(ctx, cmd, util.BufioScannerSplit(0), func(ctx gitcmd.Context, chunk []byte) error {
		if len(chunk) > 0 {
			count++
		}
		return nil
	})
	return count, err
}

type DiffShortStat struct {
	NumFiles, TotalAddition, TotalDeletion int // these fields are used in templates directly
}

func GetDiffShortStat(ctx context.Context, repo RepositoryFacade, base, head string, whitespaceBehavior gitcmd.TrustedCmdArgs) (*DiffShortStat, error) {
	stat := &DiffShortStat{}
	// output: "  9902 files changed, 2034198 insertions(+), 298800 deletions(-)\n"
	stdout, _, err := gitcmd.NewCommand("diff", "--shortstat").
		AddArguments(whitespaceBehavior...).
		AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold).
		AddDynamicArguments(base, head).WithRepo(repo).RunStdString(ctx)
	if err != nil {
		return nil, err
	}
	for field := range strings.SplitSeq(stdout, ",") {
		switch num, suffix, ok := strings.Cut(strings.TrimSpace(field), " "); {
		case strings.Contains(suffix, "file") && strings.Contains(suffix, "change"):
			stat.NumFiles, _ = strconv.Atoi(num)
		case strings.Contains(suffix, "insertion"):
			stat.TotalAddition, _ = strconv.Atoi(num)
		case strings.Contains(suffix, "deletion"):
			stat.TotalDeletion, _ = strconv.Atoi(num)
		case ok:
			setting.PanicInDevOrTesting("unexpected diff shortstat output: %s", stdout)
		}
	}
	return stat, nil
}

// GetDiff generates and returns patch data between given revisions, optimized for human readability
func (repo *Repository) GetDiff(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p").AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		RunWithStderr(ctx)
}

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func (repo *Repository) GetDiffBinary(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p", "--binary", "--histogram").
		AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		RunWithStderr(ctx)
}

// GetPatch generates and returns format-patch data between given revisions, able to be used with `git apply`
func (repo *Repository) GetPatch(ctx context.Context, compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("format-patch", "--binary", "--stdout").AddDynamicArguments(compareArg).
		WithRepo(repo).
		WithStdoutCopy(w).
		RunWithStderr(ctx)
}

// GetFilesChangedBetween returns a list of all files that have been changed between the given commits.
// If base is undefined empty SHA (zeros), it only returns the files changed in the head commit.
// If base is the SHA of an empty tree (EmptyTreeSHA), it returns the files changes from the initial commit to the head commit.
func (repo *Repository) GetFilesChangedBetween(ctx context.Context, base, head string) ([]string, error) {
	return getFilesChangedBetweenWithEnv(ctx, repo, base, head, nil)
}

func getFilesChangedBetweenWithEnv(ctx context.Context, repo RepositoryFacade, base, head string, env []string) (ret []string, _ error) {
	// don't use rename detection: when there are branch-protection or code-owner rules, old name and new name both should be checked
	cmd := gitcmd.NewCommand("diff-tree", "--name-only", "--root", "--no-commit-id", "-r", "-z").WithRepo(repo).WithEnv(env)
	if IsEmptyObjectID(base) {
		cmd.AddDynamicArguments(head)
	} else {
		cmd.AddDynamicArguments(base, head)
	}
	err := gitcmd.RunWithScannerSplit(ctx, cmd, util.BufioScannerSplit(0), func(ctx gitcmd.Context, chunk []byte) error {
		// Because Git will always emit filenames with a terminal NUL ignore the last entry in the split - which will always be empty.
		if len(chunk) > 0 {
			ret = append(ret, string(chunk))
		}
		return nil
	})
	return ret, err
}

// ReadPatchCommit will check if a diff patch exists and return stats
func (repo *Repository) ReadPatchCommit(prID int64) (commitSHA string, err error) {
	// Migrated repositories download patches to "pulls" location
	repoFS := gitrepo.RepoLocalFS(repo)
	loadPatch, err := repoFS.Open(fmt.Sprintf("pulls/%d.patch", prID))
	if err != nil {
		return "", err
	}
	defer loadPatch.Close()
	// Read only the first line of the patch - usually it contains the first commit made in patch
	scanner := bufio.NewScanner(loadPatch)
	scanner.Scan()
	// Parse the Patch stats, sometimes Migration returns a 404 for the patch file
	commitSHAGroups := globalVars().patchCommits.FindStringSubmatch(scanner.Text())
	if len(commitSHAGroups) != 0 {
		commitSHA = commitSHAGroups[1]
	} else {
		return "", errors.New("patch file doesn't contain valid commit ID")
	}
	return commitSHA, nil
}
