// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func UpdateRef(ctx context.Context, repo Repository, refName, newCommitID string) error {
	_, _, err := gitcmd.NewCommand("update-ref", "--no-deref").AddDynamicArguments(refName, newCommitID).RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return err
}

func RemoveRef(ctx context.Context, repo Repository, refName string) error {
	_, _, err := gitcmd.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(refName).RunStdString(ctx, &gitcmd.RunOpts{Dir: repoPath(repo)})
	return err
}
