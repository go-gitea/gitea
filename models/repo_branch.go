// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
)

// discardLocalRepoBranchChanges discards local commits/changes of
// given branch to make sure it is even to remote branch.
func discardLocalRepoBranchChanges(localPath, branch string) error {
	if !com.IsExist(localPath) {
		return nil
	}
	// No need to check if nothing in the repository.
	if !git.IsBranchExist(localPath, branch) {
		return nil
	}

	refName := "origin/" + branch
	if err := git.ResetHEAD(localPath, true, refName); err != nil {
		return fmt.Errorf("git reset --hard %s: %v", refName, err)
	}
	return nil
}

// DiscardLocalRepoBranchChanges discards the local repository branch changes
func (repo *Repository) DiscardLocalRepoBranchChanges(branch string) error {
	return discardLocalRepoBranchChanges(repo.LocalCopyPath(), branch)
}

// checkoutNewBranch checks out to a new branch from the a branch name.
func checkoutNewBranch(repoPath, localPath, oldBranch, newBranch string) error {
	if err := git.Checkout(localPath, git.CheckoutOptions{
		Timeout:   time.Duration(setting.Git.Timeout.Pull) * time.Second,
		Branch:    newBranch,
		OldBranch: oldBranch,
	}); err != nil {
		return fmt.Errorf("git checkout -b %s %s: %v", newBranch, oldBranch, err)
	}
	return nil
}

// CheckoutNewBranch checks out a new branch
func (repo *Repository) CheckoutNewBranch(oldBranch, newBranch string) error {
	return checkoutNewBranch(repo.RepoPath(), repo.LocalCopyPath(), oldBranch, newBranch)
}

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
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	// Check if branch name can be used
	if err := repo.CheckBranchName(branchName); err != nil {
		return err
	}

	localPath := repo.LocalCopyPath()

	if err = discardLocalRepoBranchChanges(localPath, oldBranchName); err != nil {
		return fmt.Errorf("discardLocalRepoChanges: %v", err)
	} else if err = repo.UpdateLocalCopyBranch(oldBranchName); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch: %v", err)
	}

	if err = repo.CheckoutNewBranch(oldBranchName, branchName); err != nil {
		return fmt.Errorf("CreateNewBranch: %v", err)
	}

	if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: branchName,
	}); err != nil {
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// updateLocalCopyToCommit pulls latest changes of given commit from repoPath to localPath.
// It creates a new clone if local copy does not exist.
// This function checks out target commit by default, it is safe to assume subsequent
// operations are operating against target commit when caller has confidence for no race condition.
func updateLocalCopyToCommit(repoPath, localPath, commit string) error {
	if !com.IsExist(localPath) {
		if err := git.Clone(repoPath, localPath, git.CloneRepoOptions{
			Timeout: time.Duration(setting.Git.Timeout.Clone) * time.Second,
		}); err != nil {
			return fmt.Errorf("git clone: %v", err)
		}
	} else {
		_, err := git.NewCommand("fetch", "origin").RunInDir(localPath)
		if err != nil {
			return fmt.Errorf("git fetch origin: %v", err)
		}
		if err := git.ResetHEAD(localPath, true, "HEAD"); err != nil {
			return fmt.Errorf("git reset --hard HEAD: %v", err)
		}
	}
	if err := git.Checkout(localPath, git.CheckoutOptions{
		Branch: commit,
	}); err != nil {
		return fmt.Errorf("git checkout %s: %v", commit, err)
	}
	return nil
}

// updateLocalCopyToCommit makes sure local copy of repository is at given commit.
func (repo *Repository) updateLocalCopyToCommit(commit string) error {
	return updateLocalCopyToCommit(repo.RepoPath(), repo.LocalCopyPath(), commit)
}

// CreateNewBranchFromCommit creates a new repository branch
func (repo *Repository) CreateNewBranchFromCommit(doer *User, commit, branchName string) (err error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	// Check if branch name can be used
	if err := repo.CheckBranchName(branchName); err != nil {
		return err
	}

	localPath := repo.LocalCopyPath()

	if err = repo.updateLocalCopyToCommit(commit); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch: %v", err)
	}

	if err = repo.CheckoutNewBranch(commit, branchName); err != nil {
		return fmt.Errorf("CheckoutNewBranch: %v", err)
	}

	if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: branchName,
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
