// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// CreateNewBranch creates a new repository branch
func CreateNewBranch(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldBranchName, branchName string) (err error) {
	// Check if branch name can be used
	if err := checkBranchName(ctx, repo, branchName); err != nil {
		return err
	}

	if !git.IsBranchExist(ctx, repo.RepoPath(), oldBranchName) {
		return git_model.ErrBranchDoesNotExist{
			BranchName: oldBranchName,
		}
	}

	if err := git.Push(ctx, repo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s%s:%s%s", git.BranchPrefix, oldBranchName, git.BranchPrefix, branchName),
		Env:    repo_module.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("push: %w", err)
	}

	return nil
}

// Branch contains the branch information
type Branch struct {
	RawBranch         *git_model.Branch
	IsProtected       bool
	IsIncluded        bool
	CommitsAhead      int
	CommitsBehind     int
	LatestPullRequest *issues_model.PullRequest
	MergeMovedOn      bool
}

// LoadBranches loads branches from the repository limited by page & pageSize.
func LoadBranches(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, isDeletedBranch util.OptionalBool, page, pageSize int) (*Branch, []*Branch, int64, error) {
	defaultRawBranch, err := git_model.GetBranch(ctx, repo.ID, repo.DefaultBranch)
	if err != nil {
		return nil, nil, 0, err
	}

	rawBranches, totalNumOfBranches, err := git_model.FindBranches(ctx, git_model.FindBranchOptions{
		RepoID:               repo.ID,
		IncludeDefaultBranch: false,
		IsDeletedBranch:      isDeletedBranch,
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
	})
	if err != nil {
		return nil, nil, 0, err
	}

	if err := rawBranches.LoadDeletedBy(ctx); err != nil {
		return nil, nil, 0, err
	}

	rules, err := git_model.FindRepoProtectedBranchRules(ctx, repo.ID)
	if err != nil {
		return nil, nil, 0, err
	}

	repoIDToRepo := map[int64]*repo_model.Repository{}
	repoIDToRepo[repo.ID] = repo

	repoIDToGitRepo := map[int64]*git.Repository{}
	repoIDToGitRepo[repo.ID] = gitRepo

	branches := make([]*Branch, 0, len(rawBranches))
	for i := range rawBranches {
		branch, err := loadOneBranch(ctx, repo, rawBranches[i], &rules, repoIDToRepo, repoIDToGitRepo)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("loadOneBranch: %v", err)
		}

		branches = append(branches, branch)
	}

	// Always add the default branch
	log.Debug("loadOneBranch: load default: '%s'", defaultRawBranch.Name)
	defaultBranch, err := loadOneBranch(ctx, repo, defaultRawBranch, &rules, repoIDToRepo, repoIDToGitRepo)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("loadOneBranch: %v", err)
	}

	return defaultBranch, branches, totalNumOfBranches, nil
}

func loadOneBranch(ctx context.Context, repo *repo_model.Repository, rawBranch *git_model.Branch, protectedBranches *git_model.ProtectedBranchRules,
	repoIDToRepo map[int64]*repo_model.Repository,
	repoIDToGitRepo map[int64]*git.Repository,
) (*Branch, error) {
	log.Trace("loadOneBranch: '%s'", rawBranch.Name)

	branchName := rawBranch.Name
	p := protectedBranches.GetFirstMatched(branchName)
	isProtected := p != nil

	divergence := &git.DivergeObject{
		Ahead:  -1,
		Behind: -1,
	}

	// it's not default branch
	if repo.DefaultBranch != rawBranch.Name && !rawBranch.IsDeleted {
		var err error
		divergence, err = files_service.CountDivergingCommits(ctx, repo, git.BranchPrefix+branchName)
		if err != nil {
			log.Error("CountDivergingCommits: %v", err)
		}
	}

	pr, err := issues_model.GetLatestPullRequestByHeadInfo(repo.ID, branchName)
	if err != nil {
		return nil, fmt.Errorf("GetLatestPullRequestByHeadInfo: %v", err)
	}
	headCommit := rawBranch.CommitSHA

	mergeMovedOn := false
	if pr != nil {
		pr.HeadRepo = repo
		if err := pr.LoadIssue(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssue: %v", err)
		}
		if repo, ok := repoIDToRepo[pr.BaseRepoID]; ok {
			pr.BaseRepo = repo
		} else if err := pr.LoadBaseRepo(ctx); err != nil {
			return nil, fmt.Errorf("LoadBaseRepo: %v", err)
		} else {
			repoIDToRepo[pr.BaseRepoID] = pr.BaseRepo
		}
		pr.Issue.Repo = pr.BaseRepo

		if pr.HasMerged {
			baseGitRepo, ok := repoIDToGitRepo[pr.BaseRepoID]
			if !ok {
				baseGitRepo, err = git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
				if err != nil {
					return nil, fmt.Errorf("OpenRepository: %v", err)
				}
				defer baseGitRepo.Close()
				repoIDToGitRepo[pr.BaseRepoID] = baseGitRepo
			}
			pullCommit, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil && !git.IsErrNotExist(err) {
				return nil, fmt.Errorf("GetBranchCommitID: %v", err)
			}
			if err == nil && headCommit != pullCommit {
				// the head has moved on from the merge - we shouldn't delete
				mergeMovedOn = true
			}
		}
	}

	isIncluded := divergence.Ahead == 0 && repo.DefaultBranch != branchName
	return &Branch{
		RawBranch:         rawBranch,
		IsProtected:       isProtected,
		IsIncluded:        isIncluded,
		CommitsAhead:      divergence.Ahead,
		CommitsBehind:     divergence.Behind,
		LatestPullRequest: pr,
		MergeMovedOn:      mergeMovedOn,
	}, nil
}

func GetBranchCommitID(ctx context.Context, repo *repo_model.Repository, branch string) (string, error) {
	return git.GetBranchCommitID(ctx, repo.RepoPath(), branch)
}

// checkBranchName validates branch name with existing repository branches
func checkBranchName(ctx context.Context, repo *repo_model.Repository, name string) error {
	_, err := git.WalkReferences(ctx, repo.RepoPath(), func(_, refName string) error {
		branchRefName := strings.TrimPrefix(refName, git.BranchPrefix)
		switch {
		case branchRefName == name:
			return git_model.ErrBranchAlreadyExists{
				BranchName: name,
			}
		// If branchRefName like a/b but we want to create a branch named a then we have a conflict
		case strings.HasPrefix(branchRefName, name+"/"):
			return git_model.ErrBranchNameConflict{
				BranchName: branchRefName,
			}
			// Conversely if branchRefName like a but we want to create a branch named a/b then we also have a conflict
		case strings.HasPrefix(name, branchRefName+"/"):
			return git_model.ErrBranchNameConflict{
				BranchName: branchRefName,
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
func CreateNewBranchFromCommit(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commit, branchName string) (err error) {
	// Check if branch name can be used
	if err := checkBranchName(ctx, repo, branchName); err != nil {
		return err
	}

	if err := git.Push(ctx, repo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", commit, git.BranchPrefix, branchName),
		Env:    repo_module.PushingEnvironment(doer, repo),
	}); err != nil {
		if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
			return err
		}
		return fmt.Errorf("push: %w", err)
	}

	return nil
}

// RenameBranch rename a branch
func RenameBranch(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, gitRepo *git.Repository, from, to string) (string, error) {
	if from == to {
		return "target_exist", nil
	}

	if gitRepo.IsBranchExist(to) {
		return "target_exist", nil
	}

	if !gitRepo.IsBranchExist(from) {
		return "from_not_exist", nil
	}

	if err := git_model.RenameBranch(ctx, repo, from, to, func(isDefault bool) error {
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
	refNameTo := git.RefNameFromBranch(to)
	refID, err := gitRepo.GetRefCommitID(refNameTo.String())
	if err != nil {
		return "", err
	}

	notification.NotifyDeleteRef(ctx, doer, repo, git.RefNameFromBranch(from))
	notification.NotifyCreateRef(ctx, doer, repo, refNameTo, refID)

	return "", nil
}

// enmuerates all branch related errors
var (
	ErrBranchIsDefault = errors.New("branch is default")
)

// DeleteBranch delete branch
func DeleteBranch(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, branchName string) error {
	if branchName == repo.DefaultBranch {
		return ErrBranchIsDefault
	}

	isProtected, err := git_model.IsBranchProtected(ctx, repo.ID, branchName)
	if err != nil {
		return err
	}
	if isProtected {
		return git_model.ErrBranchIsProtected
	}

	rawBranch, err := git_model.GetBranch(ctx, repo.ID, branchName)
	if err != nil {
		return fmt.Errorf("GetBranch: %vc", err)
	}

	if rawBranch.IsDeleted {
		return nil
	}

	commit, err := gitRepo.GetBranchCommit(branchName)
	if err != nil {
		return err
	}

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := git_model.AddDeletedBranch(ctx, repo.ID, branchName, doer.ID); err != nil {
			return err
		}

		return gitRepo.DeleteBranch(branchName, git.DeleteBranchOptions{
			Force: true,
		})
	}); err != nil {
		return err
	}

	// Don't return error below this
	if err := PushUpdate(
		&repo_module.PushUpdateOptions{
			RefFullName:  git.RefNameFromBranch(branchName),
			OldCommitID:  commit.ID.String(),
			NewCommitID:  git.EmptySHA,
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.OwnerName,
			RepoName:     repo.Name,
		}); err != nil {
		log.Error("Update: %v", err)
	}

	return nil
}
