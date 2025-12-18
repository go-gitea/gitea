// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func UpdateRef(ctx context.Context, repo Repository, refName, newCommitID string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("update-ref").AddDynamicArguments(refName, newCommitID))
}

func RemoveRef(ctx context.Context, repo Repository, refName string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(refName))
}
