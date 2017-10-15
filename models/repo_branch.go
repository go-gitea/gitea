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

// CreateNewBranch creates a new repository branch
func (repo *Repository) CreateNewBranch(doer *User, oldBranchName, branchName string) (err error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

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
