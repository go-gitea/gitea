// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/git"
)

// Branch holds the branch information
type Branch struct {
	Path string
	Name string
}

// GetBranchesByPath returns a branch by it's path
func GetBranchesByPath(path string) ([]*Branch, error) {
	gitRepo, err := git.OpenRepository(path)
	if err != nil {
		return nil, err
	}

	brs, err := gitRepo.GetBranches()
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(brs))
	for i := range brs {
		branches[i] = &Branch{
			Path: path,
			Name: brs[i],
		}
	}
	return branches, nil
}

// GetBranch returns a branch by it's name
func (repo *Repository) GetBranch(branch string) (*Branch, error) {
	if !git.IsBranchExist(repo.RepoPath(), branch) {
		return nil, ErrBranchNotExist{branch}
	}
	return &Branch{
		Path: repo.RepoPath(),
		Name: branch,
	}, nil
}

// GetBranches returns all the branches of a repository
func (repo *Repository) GetBranches() ([]*Branch, error) {
	return GetBranchesByPath(repo.RepoPath())
}

// GetCommit returns all the commits of a branch
func (branch *Branch) GetCommit() (*git.Commit, error) {
	gitRepo, err := git.OpenRepository(branch.Path)
	if err != nil {
		return nil, err
	}
	return gitRepo.GetBranchCommit(branch.Name)
}
