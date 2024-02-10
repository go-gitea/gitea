// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/git"
)

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repo Repository, timeout time.Duration, args git.TrustedCmdArgs) error {
	cmd := git.NewCommand(ctx, "fsck").AddArguments(args...)
	return RunGitCmd(repo, cmd, &RunOpts{
		RunOpts: git.RunOpts{Timeout: timeout},
	})
}
