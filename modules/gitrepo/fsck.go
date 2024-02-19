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
	return git.NewCommand(ctx, "fsck").AddArguments(args...).Run(&git.RunOpts{Timeout: timeout, Dir: repoPath(repo)})
}
