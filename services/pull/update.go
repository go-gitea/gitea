// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// Update updates pull request with base branch.
func Update(pull *models.PullRequest, doer *models.User, message string, rebase bool) error {
	var (
		pr    *models.PullRequest
		style models.MergeStyle
	)

	if rebase {
		pr = pull
		style = models.MergeStyleRebaseUpdate
	} else {
		//use merge functions but switch repo's and branch's
		pr = &models.PullRequest{
			HeadRepoID: pull.BaseRepoID,
			BaseRepoID: pull.HeadRepoID,
			HeadBranch: pull.BaseBranch,
			BaseBranch: pull.HeadBranch,
		}
		style = models.MergeStyleMerge
	}

	if pull.Flow == models.PullRequestFlowAGit {
		// TODO: Not support update agit flow pull request's head branch
		return fmt.Errorf("Not support update agit flow pull request's head branch")
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	diffCount, err := GetDiverging(pull)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pull.Index)
	}

	_, err = rawMerge(pr, doer, style, message)

	defer func() {
		if rebase {
			go AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false, "", "")
			return
		}
		go AddTestPullRequestTask(doer, pr.HeadRepo.ID, pr.HeadBranch, false, "", "")
	}()

	return err
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
func IsUserAllowedToUpdate(pull *models.PullRequest, user *models.User) (mergeAllowed, rebaseAllowed bool, err error) {
	if pull.Flow == models.PullRequestFlowAGit {
		return false, false, nil
	}

	if user == nil {
		return false, false, nil
	}
	headRepoPerm, err := models.GetUserRepoPermission(pull.HeadRepo, user)
	if err != nil {
		return false, false, err
	}

	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	err = pr.LoadProtectedBranch()
	if err != nil {
		return false, false, err
	}

	// can't do rebase on protected branch because need force push
	if pr.ProtectedBranch == nil {
		rebaseAllowed = true
	}

	// Update function need push permission
	if pr.ProtectedBranch != nil && !pr.ProtectedBranch.CanUserPush(user.ID) {
		return false, false, nil
	}

	mergeAllowed, err = IsUserAllowedToMerge(pr, headRepoPerm, user)
	if err != nil {
		return false, false, err
	}

	return mergeAllowed, rebaseAllowed, nil
}

// GetDiverging determines how many commits a PR is ahead or behind the PR base branch
func GetDiverging(pr *models.PullRequest) (*git.DivergeObject, error) {
	log.Trace("GetDiverging[%d]: compare commits", pr.ID)
	if err := pr.LoadBaseRepo(); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepo(); err != nil {
		return nil, err
	}

	tmpRepo, err := createTemporaryRepo(pr)
	if err != nil {
		if !models.IsErrBranchDoesNotExist(err) {
			log.Error("CreateTemporaryRepo: %v", err)
		}
		return nil, err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpRepo); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	diff, err := git.GetDivergingCommits(tmpRepo, "base", "tracking")
	return &diff, err
}
