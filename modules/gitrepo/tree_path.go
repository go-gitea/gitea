// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

func GetTreePathLatestCommit(ctx context.Context, repo Repository, commitID, treePath string) (*git.Commit, error) {
	gitRepo, closer, err := RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	stdout, _, err := git.NewCommand(ctx, "rev-list", "-1").
		AddDynamicArguments(commitID).AddArguments("--").AddDynamicArguments(treePath).
		RunStdString(&git.RunOpts{Dir: repoPath(repo)})
	if err != nil {
		return nil, err
	}

	return gitRepo.GetCommit(stdout)
}
