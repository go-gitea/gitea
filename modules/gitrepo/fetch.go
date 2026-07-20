// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
)

// FetchRemoteCommit fetches a specific commit and its related objects from a remote
// repository into the managed repository.
//
// If no reference (branch, tag, or other ref) points to the fetched commit, it will
// be treated as unreachable and cleaned up by `git gc` after the default prune
// expiration period (2 weeks). Ref: https://www.kernel.org/pub/software/scm/git/docs/git-gc.html
//
// This behavior is sufficient for temporary operations, such as determining the
// merge base between commits.
func FetchRemoteCommit(ctx context.Context, repo, remoteRepo git.RepositoryFacade, commitID string) error {
	return git.LockWriteAndDo(ctx, repo, func(ctx context.Context) error {
		return gitcmd.NewCommand("fetch", "--no-tags").
			AddDynamicArguments(repoPath(remoteRepo)).
			AddDynamicArguments(commitID).
			WithRepo(repo).Run(ctx)
	})
}
