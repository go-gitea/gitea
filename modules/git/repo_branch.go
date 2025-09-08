// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"strings"
)

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(ctx context.Context, repoPath, name string) bool {
	_, _, err := NewCommand("show-ref", "--verify").AddDashesAndList(name).RunStdString(ctx, &RunOpts{Dir: repoPath})
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(ctx context.Context, repoPath, name string) bool {
	return IsReferenceExist(ctx, repoPath, BranchPrefix+name)
}

func GetDefaultBranch(ctx context.Context, repoPath string) (string, error) {
	stdout, _, err := NewCommand("symbolic-ref", "HEAD").RunStdString(ctx, &RunOpts{Dir: repoPath})
	if err != nil {
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if !strings.HasPrefix(stdout, BranchPrefix) {
		return "", errors.New("the HEAD is not a branch: " + stdout)
	}
	return strings.TrimPrefix(stdout, BranchPrefix), nil
}

// DeleteBranchOptions Option(s) for delete branch
type DeleteBranchOptions struct {
	Force bool
}

// DeleteBranch delete a branch by name on repository.
func (repo *Repository) DeleteBranch(name string, opts DeleteBranchOptions) error {
	cmd := NewCommand("branch")

	if opts.Force {
		cmd.AddArguments("-D")
	} else {
		cmd.AddArguments("-d")
	}

	cmd.AddDashesAndList(name)
	_, _, err := cmd.RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})

	return err
}

// CreateBranch create a new branch
func (repo *Repository) CreateBranch(branch, oldbranchOrCommit string) error {
	cmd := NewCommand("branch")
	cmd.AddDashesAndList(branch, oldbranchOrCommit)

	_, _, err := cmd.RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})

	return err
}

// AddRemote adds a new remote to repository.
func (repo *Repository) AddRemote(name, url string, fetch bool) error {
	cmd := NewCommand("remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	cmd.AddDynamicArguments(name, url)

	_, _, err := cmd.RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	return err
}

// RenameBranch rename a branch
func (repo *Repository) RenameBranch(from, to string) error {
	_, _, err := NewCommand("branch", "-m").AddDynamicArguments(from, to).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	return err
}
