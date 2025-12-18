// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// MergeBase checks and returns merge base of two commits.
func MergeBase(ctx context.Context, repo Repository, commit1, commit2 string) (string, error) {
	mergeBase, err := RunCmdString(ctx, repo, gitcmd.NewCommand("merge-base").
		AddDashesAndList(commit1, commit2))
	if err != nil {
		return "", fmt.Errorf("get merge-base of %s and %s failed: %w", commit1, commit2, err)
	}
	return strings.TrimSpace(mergeBase), nil
}

// MergeBaseFromRemote checks and returns merge base of two commits from different repositories.
func MergeBaseFromRemote(ctx context.Context, repo, remoteRepo Repository, commit1, commit2 string) (string, error) {
	// fetch head commit id into the current repository if the repositories are different
	if repo.RelativePath() != remoteRepo.RelativePath() {
		if err := FetchRemoteCommit(ctx, repo, remoteRepo, commit2); err != nil {
			return "", fmt.Errorf("FetchRemoteCommit: %w", err)
		}
	}

	return MergeBase(ctx, repo, commit1, commit2)
}
