// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func UpdateRef(ctx context.Context, repo Repository, refName, newCommitID string) error {
	return UpdateRefWithOld(ctx, repo, refName, newCommitID, "")
}

// UpdateRefWithOld updates ref only when the current commit ID matches oldCommitID.
// When oldCommitID is empty, the update is unconditional.
func UpdateRefWithOld(ctx context.Context, repo Repository, refName, newCommitID, oldCommitID string) error {
	cmd := gitcmd.NewCommand("update-ref").AddDynamicArguments(refName, newCommitID)
	if oldCommitID != "" {
		cmd.AddDynamicArguments(oldCommitID)
	}
	return RunCmd(ctx, repo, cmd)
}

func RemoveRef(ctx context.Context, repo Repository, refName string) error {
	return RunCmd(ctx, repo, gitcmd.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(refName))
}
