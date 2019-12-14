// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// CanCreateBranch returns true if repository meets the requirements for creating new branches.
func (repo *Repository) CanCreateBranch() bool {
	return !repo.IsMirror
}

// GetBranch returns a branch by its name
func (repo *Repository) GetBranch(branch string) (*git.Branch, error) {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranch(branch)
}

// GetBranches returns all the branches of a repository
func (repo *Repository) GetBranches() ([]*git.Branch, error) {
	return git.GetBranchesByPath(repo.RepoPath())
}

// CheckBranchName validates branch name with existing repository branches
func (repo *Repository) CheckBranchName(name string) error {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

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
		return fmt.Errorf("OldBranch: %s does not exist. Cannot create new branch from this", oldBranchName)
	}

	basePath, err := CreateTemporaryPath("branch-maker")
	if err != nil {
		return err
	}
	defer func() {
		if err := RemoveTemporaryPath(basePath); err != nil {
			log.Error("CreateNewBranch: RemoveTemporaryPath: %s", err)
		}
	}()

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
	defer gitRepo.Close()

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
	defer func() {
		if err := RemoveTemporaryPath(basePath); err != nil {
			log.Error("CreateNewBranchFromCommit: RemoveTemporaryPath: %s", err)
		}
	}()

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
	defer gitRepo.Close()

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
