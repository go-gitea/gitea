// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func FetchRemoteCommit(ctx context.Context, repo, remoteRepo Repository, commitID string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("fetch", "--no-tags").
		AddDynamicArguments(repoPath(remoteRepo)).
		AddDynamicArguments(commitID))
}
