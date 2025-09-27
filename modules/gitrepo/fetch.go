// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func FetchRemoteCommit(ctx context.Context, repo, remoteRepo Repository, commitID string) error {
	_, _, err := gitcmd.NewCommand("fetch", "--no-tags").
		AddDynamicArguments(repoPath(remoteRepo)).
		AddDynamicArguments(commitID).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return err
}
