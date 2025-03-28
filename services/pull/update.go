// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
)

// Update updates pull request with base branch.
func Update(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, message string, rebase bool) error {
	if pr.Flow == issues_model.PullRequestFlowAGit {
		// TODO: update of agit flow pull request's head branch is unsupported
		return fmt.Errorf("update of agit flow pull request's head branch is unsupported")
	}

	releaser, err := globallock.Lock(ctx, getPullWorkingLockKey(pr.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	diffCount, err := GetDiverging(ctx, pr)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pr.Index)
	}

	if rebase {
		defer func() {
			go AddTestPullRequestTask(TestPullRequestOptions{
				RepoID:      pr.BaseRepo.ID,
				Doer:        doer,
				Branch:      pr.BaseBranch,
				IsSync:      false,
				IsForcePush: false,
				OldCommitID: "",
				NewCommitID: "",
			})
		}()

		return updateHeadByRebaseOnToBase(ctx, pr, doer)
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("unable to load BaseRepo for %-v during update-by-merge: %v", pr, err)
		return fmt.Errorf("unable to load BaseRepo for PR[%d] during update-by-merge: %w", pr.ID, err)
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("unable to load HeadRepo for PR %-v during update-by-merge: %v", pr, err)
		return fmt.Errorf("unable to load HeadRepo for PR[%d] during update-by-merge: %w", pr.ID, err)
	}
	if pr.HeadRepo == nil {
		// LoadHeadRepo will swallow ErrRepoNotExist so if pr.HeadRepo is still nil recreate the error
		err := repo_model.ErrRepoNotExist{
			ID: pr.HeadRepoID,
		}
		log.Error("unable to load HeadRepo for PR %-v during update-by-merge: %v", pr, err)
		return fmt.Errorf("unable to load HeadRepo for PR[%d] during update-by-merge: %w", pr.ID, err)
	}

	// TODO: FakePR: it is somewhat hacky, but it is the only way to "merge" at the moment
	// ideally in the future the "merge" functions should be refactored to decouple from the PullRequest
	// now use a fake reverse PR to switch head&base repos/branches
	reversePR := &issues_model.PullRequest{
		ID: pr.ID,

		HeadRepoID: pr.BaseRepoID,
		HeadRepo:   pr.BaseRepo,
		HeadBranch: pr.BaseBranch,

		BaseRepoID: pr.HeadRepoID,
		BaseRepo:   pr.HeadRepo,
		BaseBranch: pr.HeadBranch,
	}

	_, err = doMergeAndPush(ctx, reversePR, doer, repo_model.MergeStyleMerge, "", message, repository.PushTriggerPRUpdateWithBase)

	defer func() {
		go AddTestPullRequestTask(TestPullRequestOptions{
			RepoID:      reversePR.HeadRepo.ID,
			Doer:        doer,
			Branch:      reversePR.HeadBranch,
			IsSync:      false,
			IsForcePush: false,
			OldCommitID: "",
			NewCommitID: "",
		})
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
		if repo_model.IsErrUnitTypeNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}

	if err := pull.LoadBaseRepo(ctx); err != nil {
		return false, false, err
	}

	pr := &issues_model.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		HeadRepo:   pull.BaseRepo,
		BaseRepoID: pull.HeadRepoID,
		BaseRepo:   pull.HeadRepo,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, false, err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return false, false, err
	}
	prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return false, false, nil
		}
		log.Error("pr.BaseRepo.GetUnit(unit.TypePullRequests): %v", err)
		return false, false, err
	}

	rebaseAllowed = prUnit.PullRequestsConfig().AllowRebaseUpdate

	// If branch protected, disable rebase unless user is whitelisted to force push (which extends regular push)
	if pb != nil {
		pb.Repo = pull.BaseRepo
		if !pb.CanUserForcePush(ctx, user) {
			rebaseAllowed = false
		}
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
	log.Trace("GetDiverging[%-v]: compare commits", pr)
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		if !git_model.IsErrBranchNotExist(err) {
			log.Error("CreateTemporaryRepoForPR %-v: %v", pr, err)
		}
		return nil, err
	}
	defer cancel()

	diff, err := git.GetDivergingCommits(ctx, prCtx.tmpBasePath, baseBranch, trackingBranch)
	return &diff, err
}
