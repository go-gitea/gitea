// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
)

// FetchRemoteTempCommit fetches a specific commit and its related objects from a remote repository
// into the managed repository for temporary use (e.g.: compare, check merge base, etc.).
//
// If no reference (branch, tag, or other ref) points to the fetched commit, it will be treated as unreachable
// and cleaned up by auto-gc (a future push) or Gitea's "git gc" cronjob after the default prune expiration period.
// Ref: https://www.kernel.org/pub/software/scm/git/docs/git-gc.html
func FetchRemoteTempCommit(ctx context.Context, repo, remoteRepo RepositoryFacade, commitID string) error {
	// since it is for temporary use, we don't need to update any ref or lock anything or maintain anything
	return fetchRemoteCommand(remoteRepo, commitID).WithRepo(repo).Run(ctx)
}

// FetchRemoteCommitUpdateRef fetches a commit from a remote repository and points refName at it
func FetchRemoteCommitUpdateRef(ctx context.Context, repo, remoteRepo RepositoryFacade, commitish, refName string) error {
	return fetchRemoteCommand(remoteRepo, "+"+commitish+":"+refName).WithRepo(repo).RunWithStderr(ctx)
}

func fetchRemoteCommand(remoteRepo RepositoryFacade, refspec string) *gitcmd.Command {
	return gitcmd.NewCommand("fetch", "--no-tags", "--no-write-commit-graph", "--no-write-fetch-head", "--no-auto-maintenance").
		AddDynamicArguments(gitrepo.RepoLocalPath(remoteRepo), refspec)
}
