// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

// CommitsCountOptions the options when counting commits
type CommitsCountOptions struct {
	Not      string
	Revision []string
	RelPath  []string
	Since    string
	Until    string
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(ctx context.Context, repo Repository, opts CommitsCountOptions) (int64, error) {
	cmd := gitcmd.NewCommand("rev-list", "--count")

	cmd.AddDynamicArguments(opts.Revision...)

	if opts.Not != "" {
		cmd.AddOptionValues("--not", opts.Not)
	}

	if len(opts.RelPath) > 0 {
		cmd.AddDashesAndList(opts.RelPath...)
	}

	stdout, _, err := cmd.WithDir(repoPath(repo)).RunStdString(ctx)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCountBetween return numbers of commits between two commits
func CommitsCountBetween(ctx context.Context, repo Repository, start, end string) (int64, error) {
	count, err := CommitsCount(ctx, repo, CommitsCountOptions{
		Revision: []string{start + ".." + end},
	})

	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// future versions of git >= 2.28 are likely to return an error if before and last have become unrelated.
		// previously it would return the results of git rev-list before last so let's try that...
		return CommitsCount(ctx, repo, CommitsCountOptions{
			Revision: []string{start, end},
		})
	}

	return count, err
}

// FileCommitsCount return the number of files at a revision
func FileCommitsCount(ctx context.Context, repo Repository, revision, file string) (int64, error) {
	return CommitsCount(ctx, repo,
		CommitsCountOptions{
			Revision: []string{revision},
			RelPath:  []string{file},
		})
}

// CommitsCountOfCommit returns number of total commits of until current revision.
func CommitsCountOfCommit(ctx context.Context, repo Repository, commitID string) (int64, error) {
	return CommitsCount(ctx, repo, CommitsCountOptions{
		Revision: []string{commitID},
	})
}

// AllCommitsCount returns count of all commits in repository
func AllCommitsCount(ctx context.Context, repo Repository, hidePRRefs bool, files ...string) (int64, error) {
	cmd := gitcmd.NewCommand("rev-list")
	if hidePRRefs {
		cmd.AddArguments("--exclude=" + git.PullPrefix + "*")
	}
	cmd.AddArguments("--all", "--count")
	if len(files) > 0 {
		cmd.AddDashesAndList(files...)
	}

	stdout, err := RunCmdString(ctx, repo, cmd)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

func GetFullCommitID(ctx context.Context, repo Repository, shortID string) (string, error) {
	return git.GetFullCommitID(ctx, repoPath(repo), shortID)
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func GetLatestCommitTime(ctx context.Context, repo Repository) (time.Time, error) {
	stdout, err := RunCmdString(ctx, repo,
		gitcmd.NewCommand("for-each-ref", "--sort=-committerdate", git.BranchPrefix, "--count", "1", "--format=%(committerdate)"))
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse("Mon Jan _2 15:04:05 2006 -0700", commitTime)
}

// IsForcePush returns true if a push from oldCommitHash to this is a force push
func IsCommitForcePush(ctx context.Context, repo Repository, newCommitID, oldCommitID string) (bool, error) {
	if oldCommitID == git.Sha1ObjectFormat.EmptyObjectID().String() || oldCommitID == git.Sha256ObjectFormat.EmptyObjectID().String() {
		return false, nil
	}

	hasPreviousCommit, err := HasPreviousCommit(ctx, repo, newCommitID, oldCommitID)
	return !hasPreviousCommit, err
}

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func HasPreviousCommit(ctx context.Context, repo Repository, newCommitID, oldCommitID string) (bool, error) {
	if newCommitID == oldCommitID {
		return false, nil
	}

	_, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base", "--is-ancestor").
		AddDynamicArguments(oldCommitID, newCommitID))
	if err == nil {
		return true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if exitError.ProcessState.ExitCode() == 1 && len(exitError.Stderr) == 0 {
			return false, nil
		}
	}
	return false, err
}

// GetBranchName gets the closest branch name (as returned by 'git name-rev --name-only')
func GetCommitBranchName(ctx context.Context, repo Repository, commitID string) (string, error) {
	cmd := gitcmd.NewCommand("name-rev")
	if git.DefaultFeatures().CheckVersionAtLeast("2.13.0") {
		cmd.AddArguments("--exclude", "refs/tags/*")
	}
	cmd.AddArguments("--name-only", "--no-undefined").AddDynamicArguments(commitID)
	data, err := RunCmdString(ctx, repo, cmd)
	if err != nil {
		// handle special case where git can not describe commit
		if strings.Contains(err.Error(), "cannot describe") {
			return "", nil
		}

		return "", err
	}

	// name-rev commitID output will be "master" or "master~12"
	return strings.SplitN(strings.TrimSpace(data), "~", 2)[0], nil
}
