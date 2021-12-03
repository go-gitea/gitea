// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	pull_service "code.gitea.io/gitea/services/pull"
)

// CreateNewBranch creates a new repository branch
func CreateNewBranch(doer *user_model.User, repo *models.Repository, oldBranchName, branchName string) (err error) {
	// Check if branch name can be used
	if err := checkBranchName(git.DefaultContext, repo, branchName); err != nil {
		return err
	}

	if !git.IsBranchExist(git.DefaultContext, repo.RepoPath(), oldBranchName) {
		return models.ErrBranchDoesNotExist{
			BranchName: oldBranchName,
		}
	}

	if err := git.Push(git.DefaultContext, repo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", oldBranchName, git.BranchPrefix, branchName),
		Env:    models.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// GetBranch returns a branch by its name
func GetBranch(repo *models.Repository, branch string) (*git.Branch, error) {
	if len(branch) == 0 {
		return nil, fmt.Errorf("GetBranch: empty string for branch")
	}
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranch(branch)
}

// GetBranches returns branches from the repository, skipping skip initial branches and
// returning at most limit branches, or all branches if limit is 0.
func GetBranches(repo *models.Repository, skip, limit int) ([]*git.Branch, int, error) {
	return git.GetBranchesByPath(repo.RepoPath(), skip, limit)
}

// checkBranchName validates branch name with existing repository branches
func checkBranchName(ctx context.Context, repo *models.Repository, name string) error {
	_, err := git.WalkReferences(ctx, repo.RepoPath(), func(refName string) error {
		switch {
		case refName == name || refName == git.BranchPrefix+name:
			return models.ErrBranchAlreadyExists{
				BranchName: name,
			}
		case strings.HasPrefix(refName, git.BranchPrefix+name+"/"):
			return models.ErrBranchNameConflict{
				BranchName: name,
			}
		case refName == git.TagPrefix+name:
			return models.ErrTagAlreadyExists{
				TagName: name,
			}
		}
		return nil
	})

	return err
}

// CreateNewBranchFromCommit creates a new repository branch
func CreateNewBranchFromCommit(doer *user_model.User, repo *models.Repository, commit, branchName string) (err error) {
	// Check if branch name can be used
	if err := checkBranchName(git.DefaultContext, repo, branchName); err != nil {
		return err
	}

	if err := git.Push(git.DefaultContext, repo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", commit, git.BranchPrefix, branchName),
		Env:    models.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("Push: %v", err)
	}

	return nil
}

// RenameBranch rename a branch
func RenameBranch(repo *models.Repository, doer *user_model.User, gitRepo *git.Repository, from, to string) (string, error) {
	if from == to {
		return "target_exist", nil
	}

	if gitRepo.IsBranchExist(to) {
		return "target_exist", nil
	}

	if !gitRepo.IsBranchExist(from) {
		return "from_not_exist", nil
	}

	if err := repo.RenameBranch(from, to, func(isDefault bool) error {
		err2 := gitRepo.RenameBranch(from, to)
		if err2 != nil {
			return err2
		}

		if isDefault {
			err2 = gitRepo.SetDefaultBranch(to)
			if err2 != nil {
				return err2
			}
		}

		return nil
	}); err != nil {
		return "", err
	}

	notification.NotifyDeleteRef(doer, repo, "branch", git.BranchPrefix+from)
	notification.NotifyCreateRef(doer, repo, "branch", git.BranchPrefix+to)

	return "", nil
}

// enmuerates all branch related errors
var (
	ErrBranchIsDefault   = errors.New("branch is default")
	ErrBranchIsProtected = errors.New("branch is protected")
)

// DeleteBranch delete branch
func DeleteBranch(doer *user_model.User, repo *models.Repository, gitRepo *git.Repository, branchName string) error {
	if branchName == repo.DefaultBranch {
		return ErrBranchIsDefault
	}

	isProtected, err := repo.IsProtectedBranch(branchName)
	if err != nil {
		return err
	}

	if isProtected {
		return ErrBranchIsProtected
	}

	commit, err := gitRepo.GetBranchCommit(branchName)
	if err != nil {
		return err
	}

	if err := gitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
		Force: true,
	}); err != nil {
		return err
	}

	if err := pull_service.CloseBranchPulls(doer, repo.ID, branchName); err != nil {
		return err
	}

	// Don't return error below this
	if err := PushUpdate(
		&repo_module.PushUpdateOptions{
			RefFullName:  git.BranchPrefix + branchName,
			OldCommitID:  commit.ID.String(),
			NewCommitID:  git.EmptySHA,
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.OwnerName,
			RepoName:     repo.Name,
		}); err != nil {
		log.Error("Update: %v", err)
	}

	if err := repo.AddDeletedBranch(branchName, commit.ID.String(), doer.ID); err != nil {
		log.Warn("AddDeletedBranch: %v", err)
	}

	return nil
}
