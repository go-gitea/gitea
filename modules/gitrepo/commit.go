// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
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

	stdout, _, err := RunCmdString(ctx, repo, cmd)
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
	stdout, _, err := RunCmdString(ctx, repo,
		gitcmd.NewCommand("for-each-ref", "--sort=-committerdate", git.BranchPrefix, "--count", "1", "--format=%(committerdate)"))
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse("Mon Jan _2 15:04:05 2006 -0700", commitTime)
}
