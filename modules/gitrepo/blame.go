// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func LineBlame(ctx context.Context, repo Repository, revision, file string, line uint) (string, error) {
	return RunCmdString(ctx, repo,
		gitcmd.NewCommand("blame").
			AddOptionFormat("-L %d,%d", line, line).
			AddOptionValues("-p", revision).
			AddDashesAndList(file))
}
