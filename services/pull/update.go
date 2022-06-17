// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
)

// Update updates pull request with base branch.
func Update(ctx context.Context, pull *issues_model.PullRequest, doer *user_model.User, message string, rebase bool) error {
	var (
		pr    *issues_model.PullRequest
		style repo_model.MergeStyle
	)

	pullWorkingPool.CheckIn(fmt.Sprint(pull.ID))
	defer pullWorkingPool.CheckOut(fmt.Sprint(pull.ID))

	if rebase {
		pr = pull
		style = repo_model.MergeStyleRebaseUpdate
	} else {
		// use merge functions but switch repo's and branch's
		pr = &issues_model.PullRequest{
			HeadRepoID: pull.BaseRepoID,
			BaseRepoID: pull.HeadRepoID,
			HeadBranch: pull.BaseBranch,
			BaseBranch: pull.HeadBranch,
		}
		style = repo_model.MergeStyleMerge
	}

	if pull.Flow == issues_model.PullRequestFlowAGit {
		// TODO: Not support update agit flow pull request's head branch
		return fmt.Errorf("Not support update agit flow pull request's head branch")
	}

	if err := pr.LoadHeadRepoCtx(ctx); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepoCtx(ctx); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	diffCount, err := GetDiverging(ctx, pull)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pull.Index)
	}

	_, err = rawMerge(ctx, pr, doer, style, "", message)

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
func IsUserAllowedToUpdate(ctx context.Context, pull *issues_model.PullRequest, user *user_model.User) (mergeAllowed, rebaseAllowed bool, err error) {
	if pull.Flow == issues_model.PullRequestFlowAGit {
		return false, false, nil
	}

	if user == nil {
		return false, false, nil
	}
	headRepoPerm, err := access_model.GetUserRepoPermission(ctx, pull.HeadRepo, user)
	if err != nil {
		return false, false, err
	}

	pr := &issues_model.PullRequest{
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
		prUnit, err := pr.BaseRepo.GetUnit(unit.TypePullRequests)
		if err != nil {
			log.Error("pr.BaseRepo.GetUnit(unit.TypePullRequests): %v", err)
			return false, false, err
		}
		rebaseAllowed = prUnit.PullRequestsConfig().AllowRebaseUpdate
	}

	// Update function need push permission
	if pr.ProtectedBranch != nil && !pr.ProtectedBranch.CanUserPush(user.ID) {
		return false, false, nil
	}

	baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, pull.BaseRepo, user)
	if err != nil {
		return false, false, err
	}

	mergeAllowed, err = IsUserAllowedToMerge(ctx, pr, headRepoPerm, user)
	if err != nil {
		return false, false, err
	}

	if pull.AllowMaintainerEdit {
		mergeAllowedMaintainer, err := IsUserAllowedToMerge(ctx, pr, baseRepoPerm, user)
		if err != nil {
			return false, false, err
		}

		mergeAllowed = mergeAllowed || mergeAllowedMaintainer
	}

	return mergeAllowed, rebaseAllowed, nil
}

// GetDiverging determines how many commits a PR is ahead or behind the PR base branch
func GetDiverging(ctx context.Context, pr *issues_model.PullRequest) (*git.DivergeObject, error) {
	log.Trace("GetDiverging[%d]: compare commits", pr.ID)
	if err := pr.LoadBaseRepoCtx(ctx); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepoCtx(ctx); err != nil {
		return nil, err
	}

	tmpRepo, err := createTemporaryRepo(ctx, pr)
	if err != nil {
		if !models.IsErrBranchDoesNotExist(err) {
			log.Error("CreateTemporaryRepo: %v", err)
		}
		return nil, err
	}
	defer func() {
		if err := repo_module.RemoveTemporaryPath(tmpRepo); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	diff, err := git.GetDivergingCommits(ctx, tmpRepo, "base", "tracking")
	return &diff, err
}
