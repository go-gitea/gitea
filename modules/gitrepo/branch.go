// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// GetBranches returns branch names by repository
// if limit = 0 it will not limit
func GetBranches(ctx context.Context, repo Repository, skip, limit int) ([]string, int, error) {
	branchNames := make([]string, 0, limit)
	countAll, err := curService.WalkReferences(ctx, repo, git.ObjectBranch, skip, limit, func(_, branchName string) error {
		branchName = strings.TrimPrefix(branchName, git.BranchPrefix)
		branchNames = append(branchNames, branchName)
		return nil
	})
	return branchNames, countAll, err
}

func GetBranchCommitID(ctx context.Context, repo Repository, branch string) (string, error) {
	gitRepo, err := curService.OpenRepository(ctx, repo)
	if err != nil {
		return "", err
	}
	defer gitRepo.Close()
	return gitRepo.GetRefCommitID(git.BranchPrefix + branch)
}

// SetDefaultBranch sets default branch of repository.
func SetDefaultBranch(ctx context.Context, repo Repository, name string) error {
	cmd := git.NewCommand(ctx, "symbolic-ref", "HEAD").
		AddDynamicArguments(git.BranchPrefix + name)
	_, _, err := RunGitCmdStdString(ctx, repo, cmd, &git.RunOpts{})
	return err
}

// GetDefaultBranch gets default branch of repository.
func GetDefaultBranch(ctx context.Context, repo Repository) (string, error) {
	cmd := git.NewCommand(ctx, "symbolic-ref", "HEAD")
	stdout, _, err := RunGitCmdStdString(ctx, repo, cmd, &git.RunOpts{})
	if err != nil {
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if !strings.HasPrefix(stdout, git.BranchPrefix) {
		return "", errors.New("the HEAD is not a branch: " + stdout)
	}
	return strings.TrimPrefix(stdout, git.BranchPrefix), nil
}

func GetWikiDefaultBranch(ctx context.Context, repo Repository) (string, error) {
	return GetDefaultBranch(ctx, wikiRepo(repo))
}
