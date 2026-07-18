// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"gitea.dev/modules/git/gitcmd"
)

// TODO: all wrappers can be removed in next PR, because cmd now can accept Repository directly.

func RunCmd(ctx context.Context, repo Repository, cmd *gitcmd.Command) error {
	return cmd.WithRepo(repo).WithParentCallerInfo().Run(ctx)
}

func RunCmdString(ctx context.Context, repo Repository, cmd *gitcmd.Command) (string, string, gitcmd.RunStdError) {
	return cmd.WithRepo(repo).WithParentCallerInfo().RunStdString(ctx)
}

func RunCmdBytes(ctx context.Context, repo Repository, cmd *gitcmd.Command) ([]byte, []byte, gitcmd.RunStdError) {
	return cmd.WithRepo(repo).WithParentCallerInfo().RunStdBytes(ctx)
}

func RunCmdWithStderr(ctx context.Context, repo Repository, cmd *gitcmd.Command) gitcmd.RunStdError {
	return cmd.WithRepo(repo).WithParentCallerInfo().RunWithStderr(ctx)
}
