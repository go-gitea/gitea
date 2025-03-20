// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

func GetCommit(ctx context.Context, repo Repository, commitID string) (*git.Commit, error) {
	gitRepo, err := git.OpenRepository(ctx, repoPath(repo))
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()
	return gitRepo.GetCommit(commitID)
}

func CommitsCount(ctx context.Context, repo Repository, ref string) (int64, error) {
	return git.CommitsCount(ctx, git.CommitsCountOptions{
		RepoPath: repoPath(repo),
		Revision: []string{ref},
	})
}
