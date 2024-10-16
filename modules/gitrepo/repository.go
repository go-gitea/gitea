// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return curService.OpenRepository(ctx, repo)
}

func OpenWikiRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return curService.OpenRepository(ctx, wikiRepo(repo))
}
