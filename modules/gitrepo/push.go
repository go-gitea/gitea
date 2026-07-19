// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"gitea.dev/modules/git"
)

// PushToExternal pushes a managed repository to an external remote.
func PushToExternal(ctx context.Context, repo git.RepositoryFacade, opts git.PushOptions) error {
	return git.Push(ctx, repoPath(repo), opts)
}

// PushManaged pushes from one managed repository to another managed repository.
func PushManaged(ctx context.Context, fromRepo, toRepo git.RepositoryFacade, opts git.PushOptions) error {
	opts.Remote = repoPath(toRepo)
	return git.Push(ctx, repoPath(fromRepo), opts)
}

// PushFromLocal pushes from a local path to a managed repository.
func PushFromLocal(ctx context.Context, fromLocalPath string, toRepo git.RepositoryFacade, opts git.PushOptions) error {
	opts.Remote = repoPath(toRepo)
	return git.Push(ctx, fromLocalPath, opts)
}
