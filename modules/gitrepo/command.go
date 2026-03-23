// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func RunCmd(ctx context.Context, repo Repository, cmd *gitcmd.Command) error {
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().Run(ctx)
}

func RunCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, string, gitcmd.RunStdError) {
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunStdString(ctx)
}

func RunCmdBytes(ctx context.Context, repo Repository, cmd *gitcmd.Command) ([]byte, []byte, gitcmd.RunStdError) {
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunStdBytes(ctx)
}

func RunCmdWithStderr(ctx context.Context, repo Repository, cmd *gitcmd.Command) gitcmd.RunStdError {
	return cmd.WithDir(repoPath(repo)).WithParentCallerInfo().RunWithStderr(ctx)
}
