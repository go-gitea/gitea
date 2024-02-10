// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// GetBranchesByPath returns a branch by its path
// if limit = 0 it will not limit
func GetBranchesByPath(ctx context.Context, repo Repository, skip, limit int) ([]*git.Branch, int, error) {
	gitRepo, err := OpenRepository(ctx, repo)
	if err != nil {
		return nil, 0, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranches(skip, limit)
}

func GetBranchCommitID(ctx context.Context, repo Repository, branch string) (string, error) {
	gitRepo, err := OpenRepository(ctx, repo)
	if err != nil {
		return "", err
	}
	defer gitRepo.Close()

	return gitRepo.GetRefCommitID(git.BranchPrefix + branch)
}

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(ctx context.Context, repo Repository, name string) bool {
	cmd := git.NewCommand(ctx, "show-ref", "--verify").AddDashesAndList(name)
	_, _, err := RunGitCmdStdString(repo, cmd, &RunOpts{})
	return err == nil
}

func IsWikiReferenceExist(ctx context.Context, repo Repository, name string) bool {
	cmd := git.NewCommand(ctx, "show-ref", "--verify").AddDashesAndList(name)
	_, _, err := RunGitCmdStdString(repo, cmd, &RunOpts{IsWiki: true})
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.BranchPrefix+name)
}

func IsWikiBranchExist(ctx context.Context, repo Repository, name string) bool {
	return IsWikiReferenceExist(ctx, repo, git.BranchPrefix+name)
}
