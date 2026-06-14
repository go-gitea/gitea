// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"gitea.dev/modules/git/gitcmd"
)

// FetchRemoteCommit fetches a specific commit and its related objects from a remote
// repository into the managed repository.
//
// If no reference (branch, tag, or other ref) points to the fetched commit, it becomes
// unreachable. It can be cleaned up by a later `git gc` / auto-gc opportunity, such as
// a future push, subject to the repository's prune policy. Ref: https://www.kernel.org/pub/software/scm/git/docs/git-gc.html
//
// This behavior is sufficient for temporary operations, such as determining the
// merge base between commits.
func FetchRemoteCommit(ctx context.Context, repo, remoteRepo Repository, commitID string) error {
	// Avoid shared FETCH_HEAD and auto-maintenance side effects because callers only need the object data.
	return RunCmd(ctx, repo, gitcmd.NewCommand("fetch", "--no-tags", "--no-write-fetch-head", "--no-auto-maintenance").
		AddDynamicArguments(repoPath(remoteRepo)).
		AddDynamicArguments(commitID))
}
