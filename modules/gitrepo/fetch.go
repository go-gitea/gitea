// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// FetchRemoteCommit fetches a specific commit and related commits from a remote repository into the managed repository
// it will be checked in 2 weeks by default from git if the pull request created failure.
// It's enough for a temporary fetch to get the merge base.
func FetchRemoteCommit(ctx context.Context, repo, remoteRepo Repository, commitID string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("fetch", "--no-tags").
		AddDynamicArguments(repoPath(remoteRepo)).
		AddDynamicArguments(commitID))
}
