// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// FetchRemoteBranch fetches a remote branch into a local branch
func FetchRemoteBranch(ctx context.Context, repo Repository, localBranch string, remoteRepo Repository, remoteBranch string, args ...string) error {
	_, _, err := gitcmd.NewCommand("fetch", "--no-tags", "--refmap=").
		AddDynamicArguments(repoPath(remoteRepo)).
		// + means force fetch
		AddDynamicArguments(fmt.Sprintf("+refs/heads/%s:%s", remoteBranch, localBranch)).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return err
}

func FetchRemoteCommit(ctx context.Context, repo, remoteRepo Repository, commitID string) error {
	_, _, err := gitcmd.NewCommand("fetch", "--no-tags").
		AddDynamicArguments(repoPath(remoteRepo)).
		AddDynamicArguments(commitID).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return err
}
