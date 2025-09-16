// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

func RemoveReference(ctx context.Context, repo Repository, refName string) error {
	_, _, err := git.NewCommand("update-ref", "--no-deref", "-d").
		AddDynamicArguments(refName).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}
