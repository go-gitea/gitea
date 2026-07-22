// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/modules/git/gitcmd"
)

// CloneExternalRepo clones an external repository to the managed repository.
func CloneExternalRepo(ctx context.Context, fromRemoteURL string, toRepo RepositoryFacade, opts CloneRepoOptions) error {
	return Clone(ctx, fromRemoteURL, gitcmd.RepoLocalPath(toRepo), opts)
}

// CloneRepoToLocal clones a managed repository to a local path.
func CloneRepoToLocal(ctx context.Context, fromRepo RepositoryFacade, toLocalPath string, opts CloneRepoOptions) error {
	return Clone(ctx, gitcmd.RepoLocalPath(fromRepo), toLocalPath, opts)
}

func CloneManaged(ctx context.Context, fromRepo, toRepo RepositoryFacade, opts CloneRepoOptions) error {
	return Clone(ctx, gitcmd.RepoLocalPath(fromRepo), gitcmd.RepoLocalPath(toRepo), opts)
}
