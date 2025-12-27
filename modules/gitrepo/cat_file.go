// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git/catfile"
)

func NewBatch(ctx context.Context, repo Repository) (catfile.Batch, error) {
	return catfile.NewBatch(ctx, repoPath(repo))
}
