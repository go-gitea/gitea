// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/git"
)

type Branch struct {
	Path string
	Name string
}

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

func (repo *Repository) GetBranch(br string) (*Branch, error) {
	if !git.IsBranchExist(repo.RepoPath(), br) {
		return nil, &ErrBranchNotExist{br}
	}
	return &Branch{
		Path: repo.RepoPath(),
		Name: br,
	}, nil
}

func (repo *Repository) GetBranches() ([]*Branch, error) {
	return GetBranchesByPath(repo.RepoPath())
}

func (br *Branch) GetCommit() (*git.Commit, error) {
	gitRepo, err := git.OpenRepository(br.Path)
	if err != nil {
		return nil, err
	}
	return gitRepo.GetBranchCommit(br.Name)
}
