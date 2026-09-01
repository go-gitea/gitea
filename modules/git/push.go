// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/modules/git/gitrepo"
)

// PushToExternal pushes a managed repository to an external remote.
func PushToExternal(ctx context.Context, repo RepositoryFacade, opts PushOptions) error {
	return Push(ctx, gitrepo.RepoLocalPath(repo), opts)
}

// PushManaged pushes from one managed repository to another managed repository.
func PushManaged(ctx context.Context, fromRepo, toRepo RepositoryFacade, opts PushOptions) error {
	opts.Remote = gitrepo.RepoLocalPath(toRepo)
	return Push(ctx, gitrepo.RepoLocalPath(fromRepo), opts)
}

// PushFromLocal pushes from a local path to a managed repository.
func PushFromLocal(ctx context.Context, fromLocalPath string, toRepo RepositoryFacade, opts PushOptions) error {
	opts.Remote = gitrepo.RepoLocalPath(toRepo)
	return Push(ctx, fromLocalPath, opts)
}
