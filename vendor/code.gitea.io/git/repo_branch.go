// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"

	"github.com/mcuadros/go-version"
)

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(repoPath, name string) bool {
	_, err := NewCommand("show-ref", "--verify", name).RunInDir(repoPath)
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(repoPath, name string) bool {
	return IsReferenceExist(repoPath, BranchPrefix+name)
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	return IsBranchExist(repo.Path, name)
}

// Branch represents a Git branch.
type Branch struct {
	Name string
	Path string
}

// GetHEADBranch returns corresponding branch of HEAD.
func (repo *Repository) GetHEADBranch() (*Branch, error) {
	stdout, err := NewCommand("symbolic-ref", "HEAD").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	stdout = strings.TrimSpace(stdout)

	if !strings.HasPrefix(stdout, BranchPrefix) {
		return nil, fmt.Errorf("invalid HEAD branch: %v", stdout)
	}

	return &Branch{
		Name: stdout[len(BranchPrefix):],
		Path: stdout,
	}, nil
}

// SetDefaultBranch sets default branch of repository.
func (repo *Repository) SetDefaultBranch(name string) error {
	if version.Compare(gitVersion, "1.7.10", "<") {
		return ErrUnsupportedVersion{"1.7.10"}
	}

	_, err := NewCommand("symbolic-ref", "HEAD", BranchPrefix+name).RunInDir(repo.Path)
	return err
}

// GetBranches returns all branches of the repository.
func (repo *Repository) GetBranches() ([]string, error) {
	stdout, err := NewCommand("for-each-ref", "--format=%(refname)", BranchPrefix).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	refs := strings.Split(stdout, "\n")
	branches := make([]string, len(refs)-1)
	for i, ref := range refs[:len(refs)-1] {
		branches[i] = strings.TrimPrefix(ref, BranchPrefix)
	}
	return branches, nil
}

// DeleteBranchOptions Option(s) for delete branch
type DeleteBranchOptions struct {
	Force bool
}

// DeleteBranch delete a branch by name on repository.
func (repo *Repository) DeleteBranch(name string, opts DeleteBranchOptions) error {
	cmd := NewCommand("branch", "-d")

	if opts.Force {
		cmd.AddArguments("-f")
	}

	cmd.AddArguments(name)
	_, err := cmd.RunInDir(repo.Path)

	return err
}

// CreateBranch create a new branch
func (repo *Repository) CreateBranch(branch, newBranch string) error {
	cmd := NewCommand("branch")
	cmd.AddArguments(branch, newBranch)

	_, err := cmd.RunInDir(repo.Path)

	return err
}

// AddRemote adds a new remote to repository.
func (repo *Repository) AddRemote(name, url string, fetch bool) error {
	cmd := NewCommand("remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	cmd.AddArguments(name, url)

	_, err := cmd.RunInDir(repo.Path)
	return err
}

// RemoveRemote removes a remote from repository.
func (repo *Repository) RemoveRemote(name string) error {
	_, err := NewCommand("remote", "remove", name).RunInDir(repo.Path)
	return err
}
