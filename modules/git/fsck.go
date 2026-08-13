// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"time"

	"gitea.dev/modules/git/gitcmd"
)

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repo RepositoryFacade, timeout time.Duration, args gitcmd.TrustedCmdArgs) error {
	return gitcmd.NewCommand("fsck").AddArguments(args...).WithTimeout(timeout).WithRepo(repo).Run(ctx)
}
