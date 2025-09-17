// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func runCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, error) {
	res, _, err := cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return res, err
}
