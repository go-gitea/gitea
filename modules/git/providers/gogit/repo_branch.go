// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing"
)

//  _
// |_) ._  _. ._   _ |_
// |_) |  (_| | | (_ | |
//

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	reference, err := repo.gogitRepo.Reference(plumbing.ReferenceName(git.BranchPrefix+name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// GetBranches returns all branches of the repository.
func (repo *Repository) GetBranches() ([]string, error) {
	var branchNames []string

	branches, err := repo.gogitRepo.Branches()
	if err != nil {
		return nil, err
	}

	_ = branches.ForEach(func(branch *plumbing.Reference) error {
		branchNames = append(branchNames, strings.TrimPrefix(branch.Name().String(), git.BranchPrefix))
		return nil
	})

	// TODO: Sort?

	return branchNames, nil
}

// SetDefaultBranch sets default branch of repository.
func (repo *Repository) SetDefaultBranch(name string) error {
	_, err := git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+name).RunInDir(repo.Path())
	return err
}

// GetDefaultBranch gets default branch of repository.
func (repo *Repository) GetDefaultBranch() (string, error) {
	return git.NewCommand("symbolic-ref", "HEAD").RunInDir(repo.Path())
}

// GetHEADBranch returns corresponding branch of HEAD.
func (repo *Repository) GetHEADBranch() (service.Branch, error) {
	if repo == nil {
		return nil, fmt.Errorf("nil repo")
	}
	stdout, err := git.NewCommand("symbolic-ref", "HEAD").RunInDir(repo.Path())
	if err != nil {
		return nil, err
	}
	stdout = strings.TrimSpace(stdout)

	if !strings.HasPrefix(stdout, git.BranchPrefix) {
		return nil, fmt.Errorf("invalid HEAD branch: %v", stdout)
	}

	return common.NewBranch(
		stdout[len(git.BranchPrefix):],
		"",
		repo,
	), nil
}

// GetBranch returns a branch by it's name
func (repo *Repository) GetBranch(branch string) (service.Branch, error) {
	if !repo.IsBranchExist(branch) {
		return nil, git.ErrBranchNotExist{
			Name: branch,
		}
	}
	return common.NewBranch(
		branch,
		"",
		repo,
	), nil
}

// DeleteBranch delete a branch by name on repository.
func (repo *Repository) DeleteBranch(name string, opts service.DeleteBranchOptions) error {
	cmd := git.NewCommand("branch")

	if opts.Force {
		cmd.AddArguments("-D")
	} else {
		cmd.AddArguments("-d")
	}

	cmd.AddArguments("--", name)
	_, err := cmd.RunInDir(repo.Path())

	return err
}

// CreateBranch create a new branch
func (repo *Repository) CreateBranch(branch, oldbranchOrCommit string) error {
	cmd := git.NewCommand("branch")
	cmd.AddArguments("--", branch, oldbranchOrCommit)

	_, err := cmd.RunInDir(repo.Path())

	return err
}
