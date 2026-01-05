// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/catfile"
)

func NewObjectPool(ctx context.Context, repo Repository) (catfile.ObjectPool, error) {
	return catfile.NewObjectPool(ctx, repoPath(repo))
}
