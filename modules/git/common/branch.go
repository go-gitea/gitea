// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
)

var _ (service.Branch) = &Branch{}

// Branch represents a Git branch.
type Branch struct {
	name string
	path string

	repo service.Repository
}

// NewBranch creates a new branch object
func NewBranch(name, path string, repo service.Repository) service.Branch {
	return &Branch{
		name: name,
		path: path,
		repo: repo,
	}
}

// Name returns the branch name
func (branch *Branch) Name() string {
	return branch.name
}

// GetCommit returns the head commit of a branch
func (branch *Branch) GetCommit() (service.Commit, error) {
	return branch.repo.GetBranchCommit(branch.name)
}

// GetBranchesByPath returns a branch by it's path
func GetBranchesByPath(path string) ([]service.Branch, error) {
	gitRepo, err := git.Service.OpenRepository(path)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	brs, err := gitRepo.GetBranches()
	if err != nil {
		return nil, err
	}

	branches := make([]service.Branch, len(brs))
	for i := range brs {
		branches[i] = NewBranch(brs[i], path, gitRepo)
	}

	return branches, nil
}
