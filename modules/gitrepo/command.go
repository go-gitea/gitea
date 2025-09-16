// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func runCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, string, error) { //nolint:unparam // the second return parameter maybe used in the future
	return cmd.RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
}
