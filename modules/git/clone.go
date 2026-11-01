// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/modules/git/gitrepo"
)

// CloneExternalRepo clones an external repository to the managed repository.
func CloneExternalRepo(ctx context.Context, fromRemoteURL string, toRepo RepositoryFacade, opts CloneRepoOptions) error {
	return GuardStorageMutation(ctx, toRepo, StorageClone, func(ctx context.Context, target RepositoryFacade, env []string) error {
		if len(env) != 0 {
			opts.Shared, opts.Dissociate = false, true
		}
		opts.Env = append(opts.Env, env...)
		return Clone(ctx, fromRemoteURL, gitrepo.RepoLocalPath(target), opts)
	})
}

// CloneRepoToLocal clones a managed repository to a local path.
func CloneRepoToLocal(ctx context.Context, fromRepo RepositoryFacade, toLocalPath string, opts CloneRepoOptions) error {
	return Clone(ctx, gitrepo.RepoLocalPath(fromRepo), toLocalPath, opts)
}

func CloneManaged(ctx context.Context, fromRepo, toRepo RepositoryFacade, opts CloneRepoOptions) error {
	return CloneExternalRepo(ctx, gitrepo.RepoLocalPath(fromRepo), toRepo, opts)
}
