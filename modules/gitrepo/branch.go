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

	return gitRepo.GetBranchCommitID(branch)
}

// SetDefaultBranchForRepo sets default branch of repository.
func SetDefaultBranchForRepo(ctx context.Context, repo Repository) error {
	_, _, err := git.NewCommand("symbolic-ref", "HEAD").
		AddDynamicArguments(git.BranchPrefix+repo.GetDefaultBranch()).
		RunStdString(ctx, &git.RunOpts{Dir: repoPath(repo)})
	return err
}

func SetDefaultBranchForWiki(ctx context.Context, repo Repository) error {
	_, _, err := git.NewCommand("symbolic-ref", "HEAD").
		AddDynamicArguments(git.BranchPrefix+repo.GetDefaultWikiBranch()).
		RunStdString(ctx, &git.RunOpts{Dir: wikiPath(repo)})
	return err
}

// GetDefaultBranch gets default branch of repository.
func GetDefaultBranch(ctx context.Context, repo Repository) (string, error) {
	return git.GetDefaultBranch(ctx, repoPath(repo))
}

func GetWikiDefaultBranch(ctx context.Context, repo Repository) (string, error) {
	return git.GetDefaultBranch(ctx, wikiPath(repo))
}

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(ctx context.Context, repo Repository, name string) bool {
	return git.IsReferenceExist(ctx, repoPath(repo), name)
}

func IsWikiReferenceExist(ctx context.Context, repo Repository, name string) bool {
	return git.IsReferenceExist(ctx, wikiPath(repo), name)
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.BranchPrefix+name)
}

func IsWikiBranchExist(ctx context.Context, repo Repository, name string) bool {
	return IsWikiReferenceExist(ctx, repo, git.BranchPrefix+name)
}
