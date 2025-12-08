// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// CloneExternalRepo clones an external repository to the managed repository.
func CloneExternalRepo(ctx context.Context, fromRemoteURL string, toRepo Repository, opts git.CloneRepoOptions) error {
	return git.Clone(ctx, fromRemoteURL, repoPath(toRepo), opts)
}

// CloneRepoToLocal clones a managed repository to a local path.
func CloneRepoToLocal(ctx context.Context, fromRepo Repository, toLocalPath string, opts git.CloneRepoOptions) error {
	return git.Clone(ctx, repoPath(fromRepo), toLocalPath, opts)
}
