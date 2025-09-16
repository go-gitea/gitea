// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func LineBlame(ctx context.Context, repo Repository, revision, file string, line uint) (string, error) {
	res, _, err := runCmdString(ctx, repo,
		gitcmd.NewCommand("blame").
			AddOptionFormat("-L %d,%d", line, line).
			AddOptionValues("-p", revision).
			AddDashesAndList(file))
	return res, err
}
