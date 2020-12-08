// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

//  _
// |_) ._  _. ._   _ |_
// |_) |  (_| | | (_ | |
//

// IsBranchExist returns true if given branch exists in current repository.
func (r *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	return git.IsReferenceExist(r.Path(), git.BranchPrefix+name)
}

// GetBranches returns all branches of the repository.
func (r *Repository) GetBranches() ([]string, error) {
	return callShowRef(r.Path(), git.BranchPrefix, "--heads")
}

// SetDefaultBranch sets default branch of repository.
func (r *Repository) SetDefaultBranch(name string) error {
	_, err := git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+name).RunInDir(r.Path())
	return err
}

// GetDefaultBranch gets default branch of repository.
func (r *Repository) GetDefaultBranch() (string, error) {
	return git.NewCommand("symbolic-ref", "HEAD").RunInDir(r.Path())
}

// GetHEADBranch returns corresponding branch of HEAD.
func (r *Repository) GetHEADBranch() (service.Branch, error) {
	if r == nil {
		return nil, fmt.Errorf("nil repo")
	}
	stdout, err := git.NewCommand("symbolic-ref", "HEAD").RunInDir(r.Path())
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
		r,
	), nil
}

// GetBranch returns a branch by it's name
func (r *Repository) GetBranch(branch string) (service.Branch, error) {
	if !r.IsBranchExist(branch) {
		return nil, git.ErrBranchNotExist{
			Name: branch,
		}
	}
	return common.NewBranch(
		branch,
		"",
		r,
	), nil
}

// DeleteBranch delete a branch by name on repository.
func (r *Repository) DeleteBranch(name string, opts service.DeleteBranchOptions) error {
	cmd := git.NewCommand("branch")

	if opts.Force {
		cmd.AddArguments("-D")
	} else {
		cmd.AddArguments("-d")
	}

	cmd.AddArguments("--", name)
	_, err := cmd.RunInDir(r.Path())

	return err
}

// CreateBranch create a new branch
func (r *Repository) CreateBranch(branch, oldbranchOrCommit string) error {
	cmd := git.NewCommand("branch")
	cmd.AddArguments("--", branch, oldbranchOrCommit)

	_, err := cmd.RunInDir(r.Path())

	return err
}
