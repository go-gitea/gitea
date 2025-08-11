// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	logger "code.gitea.io/gitea/modules/log"
)

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	MergeBase    string
	BaseCommitID string
	HeadCommitID string
	Commits      []*Commit
	NumFiles     int
}

// GetMergeBase checks and returns merge base of two branches and the reference used as base.
func (repo *Repository) GetMergeBase(tmpRemote, base, head string) (string, string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := RemotePrefix + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, _, err := NewCommand("fetch", "--no-tags").AddDynamicArguments(tmpRemote).AddDashesAndList(base+":"+tmpBaseName).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, _, err := NewCommand("merge-base").AddDashesAndList(base, head).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	return strings.TrimSpace(stdout), base, err
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func (repo *Repository) GetCompareInfo(basePath, baseBranch, headBranch string, directComparison, fileOnly bool) (_ *CompareInfo, err error) {
	var (
		remoteBranch string
		tmpRemote    string
	)

	// We don't need a temporary remote for same repository.
	if repo.Path != basePath {
		// Add a temporary remote
		tmpRemote = strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = repo.AddRemote(tmpRemote, basePath, false); err != nil {
			return nil, fmt.Errorf("AddRemote: %w", err)
		}
		defer func() {
			if err := repo.RemoveRemote(tmpRemote); err != nil {
				logger.Error("GetPullRequestInfo: RemoveRemote: %v", err)
			}
		}()
	}

	compareInfo := new(CompareInfo)

	compareInfo.HeadCommitID, err = GetFullCommitID(repo.Ctx, repo.Path, headBranch)
	if err != nil {
		compareInfo.HeadCommitID = headBranch
	}

	compareInfo.MergeBase, remoteBranch, err = repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
		compareInfo.BaseCommitID, err = GetFullCommitID(repo.Ctx, repo.Path, remoteBranch)
		if err != nil {
			compareInfo.BaseCommitID = remoteBranch
		}
		separator := "..."
		baseCommitID := compareInfo.MergeBase
		if directComparison {
			separator = ".."
			baseCommitID = compareInfo.BaseCommitID
		}

		// We have a common base - therefore we know that ... should work
		if !fileOnly {
			// avoid: ambiguous argument 'refs/a...refs/b': unknown revision or path not in the working tree. Use '--': 'git <command> [<revision>...] -- [<file>...]'
			var logs []byte
			logs, _, err = NewCommand("log").AddArguments(prettyLogFormat).
				AddDynamicArguments(baseCommitID+separator+headBranch).AddArguments("--").
				RunStdBytes(repo.Ctx, &RunOpts{Dir: repo.Path})
			if err != nil {
				return nil, err
			}
			compareInfo.Commits, err = repo.parsePrettyFormatLogToList(logs)
			if err != nil {
				return nil, fmt.Errorf("parsePrettyFormatLogToList: %w", err)
			}
		} else {
			compareInfo.Commits = []*Commit{}
		}
	} else {
		compareInfo.Commits = []*Commit{}
		compareInfo.MergeBase, err = GetFullCommitID(repo.Ctx, repo.Path, remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
		compareInfo.BaseCommitID = compareInfo.MergeBase
	}

	// Count number of changed files.
	// This probably should be removed as we need to use shortstat elsewhere
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	compareInfo.NumFiles, err = repo.GetDiffNumChangedFiles(remoteBranch, headBranch, directComparison)
	if err != nil {
		return nil, err
	}
	return compareInfo, nil
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
	if err := NewCommand("diff", "-z", "--name-only").AddDynamicArguments(base+separator+head).AddArguments("--").
		Run(repo.Ctx, &RunOpts{
			Dir:    repo.Path,
			Stdout: w,
			Stderr: stderr,
		}); err != nil {
		if strings.Contains(stderr.String(), "no merge base") {
			// git >= 2.28 now returns an error if base and head have become unrelated.
			// previously it would return the results of git diff -z --name-only base head so let's try that...
			w = &lineCountWriter{}
			stderr.Reset()
			if err = NewCommand("diff", "-z", "--name-only").AddDynamicArguments(base, head).AddArguments("--").Run(repo.Ctx, &RunOpts{
				Dir:    repo.Path,
				Stdout: w,
				Stderr: stderr,
			}); err == nil {
				return w.numLines, nil
			}
		}
		return 0, fmt.Errorf("%w: Stderr: %s", err, stderr)
	}
	return w.numLines, nil
}

// GetDiffShortStatByCmdArgs counts number of changed files, number of additions and deletions
// TODO: it can be merged with another "GetDiffShortStat" in the future
func GetDiffShortStatByCmdArgs(ctx context.Context, repoPath string, trustedArgs TrustedCmdArgs, dynamicArgs ...string) (numFiles, totalAdditions, totalDeletions int, err error) {
	// Now if we call:
	// $ git diff --shortstat 1ebb35b98889ff77299f24d82da426b434b0cca0...788b8b1440462d477f45b0088875
	// we get:
	// " 9902 files changed, 2034198 insertions(+), 298800 deletions(-)\n"
	cmd := NewCommand("diff", "--shortstat").AddArguments(trustedArgs...).AddDynamicArguments(dynamicArgs...)
	stdout, _, err := cmd.RunStdString(ctx, &RunOpts{Dir: repoPath})
	if err != nil {
		return 0, 0, 0, err
	}

	return parseDiffStat(stdout)
}

var shortStatFormat = regexp.MustCompile(
	`\s*(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

var patchCommits = regexp.MustCompile(`^From\s(\w+)\s`)

func parseDiffStat(stdout string) (numFiles, totalAdditions, totalDeletions int, err error) {
	if len(stdout) == 0 || stdout == "\n" {
		return 0, 0, 0, nil
	}
	groups := shortStatFormat.FindStringSubmatch(stdout)
	if len(groups) != 4 {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s groups: %s", stdout, groups)
	}

	numFiles, err = strconv.Atoi(groups[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumFiles %w", stdout, err)
	}

	if len(groups[2]) != 0 {
		totalAdditions, err = strconv.Atoi(groups[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumAdditions %w", stdout, err)
		}
	}

	if len(groups[3]) != 0 {
		totalDeletions, err = strconv.Atoi(groups[3])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumDeletions %w", stdout, err)
		}
	}
	return numFiles, totalAdditions, totalDeletions, err
}

// GetDiff generates and returns patch data between given revisions, optimized for human readability
func (repo *Repository) GetDiff(compareArg string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	return NewCommand("diff", "-p").AddDynamicArguments(compareArg).
		Run(repo.Ctx, &RunOpts{
			Dir:    repo.Path,
			Stdout: w,
			Stderr: stderr,
		})
}

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func (repo *Repository) GetDiffBinary(compareArg string, w io.Writer) error {
	return NewCommand("diff", "-p", "--binary", "--histogram").AddDynamicArguments(compareArg).Run(repo.Ctx, &RunOpts{
		Dir:    repo.Path,
		Stdout: w,
	})
}

// GetPatch generates and returns format-patch data between given revisions, able to be used with `git apply`
func (repo *Repository) GetPatch(compareArg string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	return NewCommand("format-patch", "--binary", "--stdout").AddDynamicArguments(compareArg).
		Run(repo.Ctx, &RunOpts{
			Dir:    repo.Path,
			Stdout: w,
			Stderr: stderr,
		})
}

// GetFilesChangedBetween returns a list of all files that have been changed between the given commits
// If base is undefined empty SHA (zeros), it only returns the files changed in the head commit
// If base is the SHA of an empty tree (EmptyTreeSHA), it returns the files changes from the initial commit to the head commit
func (repo *Repository) GetFilesChangedBetween(base, head string) ([]string, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	cmd := NewCommand("diff-tree", "--name-only", "--root", "--no-commit-id", "-r", "-z")
	if base == objectFormat.EmptyObjectID().String() {
		cmd.AddDynamicArguments(head)
	} else {
		cmd.AddDynamicArguments(base, head)
	}
	stdout, _, err := cmd.RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
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
