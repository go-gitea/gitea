// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
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

// CanCreateBranch returns true if repository meets the requirements for creating new branches.
func (repo *Repository) CanCreateBranch() bool {
	return !repo.IsMirror
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

// CheckBranchName validates branch name with existing repository branches
func (repo *Repository) CheckBranchName(name string) error {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return err
	}

	branches, err := repo.GetBranches()
	if err != nil {
		return err
	}

	for _, branch := range branches {
		if branch.Name == name {
			return ErrBranchAlreadyExists{branch.Name}
		} else if (len(branch.Name) < len(name) && branch.Name+"/" == name[0:len(branch.Name)+1]) ||
			(len(branch.Name) > len(name) && name+"/" == branch.Name[0:len(name)+1]) {
			return ErrBranchNameConflict{branch.Name}
		}
	}

	if _, err := gitRepo.GetTag(name); err == nil {
		return ErrTagAlreadyExists{name}
	}

	return nil
}

// CreateNewBranch creates a new repository branch
func (repo *Repository) CreateNewBranch(doer *User, oldBranchName, branchName string) (err error) {
	// Check if branch name can be used
	if err := repo.CheckBranchName(branchName); err != nil {
		return err
	}

	if !git.IsBranchExist(repo.RepoPath(), oldBranchName) {
		return ErrBranchNotExist{
			Name: oldBranchName,
		}
	}

	basePath, err := CreateTemporaryPath("branch-maker")
	if err != nil {
		return err
	}
	defer RemoveTemporaryPath(basePath)

	if err := git.Clone(repo.RepoPath(), basePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}

	if err = gitRepo.CreateBranch(branchName, oldBranchName); err != nil {
		log.Error("Unable to create branch: %s from %s. (%v)", branchName, oldBranchName, err)
		return fmt.Errorf("Unable to create branch: %s from %s. (%v)", branchName, oldBranchName, err)
	}

	if err = git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: branchName,
		Env:    PushingEnvironment(doer, repo),
	}); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// CreateNewBranchFromCommit creates a new repository branch
func (repo *Repository) CreateNewBranchFromCommit(doer *User, commit, branchName string) (err error) {
	// Check if branch name can be used
	if err := repo.CheckBranchName(branchName); err != nil {
		return err
	}
	basePath, err := CreateTemporaryPath("branch-maker")
	if err != nil {
		return err
	}
	defer RemoveTemporaryPath(basePath)

	if err := git.Clone(repo.RepoPath(), basePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", repo.FullName(), err)
	}

	gitRepo, err := git.OpenRepository(basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", basePath, err)
	}

	if err = gitRepo.CreateBranch(branchName, commit); err != nil {
		log.Error("Unable to create branch: %s from %s. (%v)", branchName, commit, err)
		return fmt.Errorf("Unable to create branch: %s from %s. (%v)", branchName, commit, err)
	}

	if err = git.Push(basePath, git.PushOptions{
		Remote: "origin",
		Branch: branchName,
		Env:    PushingEnvironment(doer, repo),
	}); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// GetCommit returns all the commits of a branch
func (branch *Branch) GetCommit() (*git.Commit, error) {
	gitRepo, err := git.OpenRepository(branch.Path)
	if err != nil {
		return nil, err
	}
	return gitRepo.GetBranchCommit(branch.Name)
}
