// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

func CloneIn(ctx context.Context, dstRepo Repository, from string, opts git.CloneRepoOptions) error {
	return git.Clone(ctx, from, repoPath(dstRepo), opts)
}

func CloneOut(ctx context.Context, fromRepo Repository, to string, opts git.CloneRepoOptions) error {
	return git.Clone(ctx, repoPath(fromRepo), to, opts)
}
