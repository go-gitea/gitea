// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
)

// Update updates pull request with base branch.
func Update(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, message string, rebase bool) error {
	if pr.Flow == issues_model.PullRequestFlowAGit {
		// TODO: update of agit flow pull request's head branch is unsupported
		return errors.New("update of agit flow pull request's head branch is unsupported")
	}

	releaser, err := globallock.Lock(ctx, getPullWorkingLockKey(pr.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("unable to load BaseRepo for %-v during update-by-merge: %v", pr, err)
		return fmt.Errorf("unable to load BaseRepo for PR[%d] during update-by-merge: %w", pr.ID, err)
	}

	// TODO: FakePR: if the PR is a fake PR (for example: from Merge Upstream), then no need to check diverging
	if pr.ID > 0 {
		diffCount, err := gitrepo.GetDivergingCommits(ctx, pr.BaseRepo, pr.BaseBranch, pr.GetGitHeadRefName())
		if err != nil {
			return err
		} else if diffCount.Behind == 0 {
			return fmt.Errorf("HeadBranch of PR %d is up to date", pr.Index)
		}
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

	defer func() {
		// The code is from https://github.com/go-gitea/gitea/pull/9784,
		// it seems a simple copy-paste from https://github.com/go-gitea/gitea/pull/7082 without a real reason.
		// TODO: DUPLICATE-PR-TASK: search and see another TODO comment for more details
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

	if rebase {
		return updateHeadByRebaseOnToBase(ctx, pr, doer)
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
	return err
}

// isUserAllowedToPushOrForcePushInRepoBranch checks whether user is allowed to push or force push in the given repo and branch
// it will check both user permission and branch protection rules
func isUserAllowedToPushOrForcePushInRepoBranch(ctx context.Context, user *user_model.User, repo *repo_model.Repository, branch string) (pushAllowed, rebaseAllowed bool, err error) {
	if user == nil {
		return false, false, nil
	}

	// 1. check user push permission on head repository
	repoPerm, err := access_model.GetUserRepoPermission(ctx, repo, user)
	if err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	pushAllowed = repoPerm.CanWrite(unit.TypeCode)

	// 2. check branch protection whether user can push or force push
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, branch)
	if err != nil {
		return false, false, err
	}
	if pb != nil { // override previous results if there is a branch protection rule
		pb.Repo = repo
		pushAllowed = pb.CanUserPush(ctx, user)
		rebaseAllowed = pb.CanUserForcePush(ctx, user)
	}
	return pushAllowed, rebaseAllowed, nil
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
// update PR means send new commits to PR head branch from base branch
func IsUserAllowedToUpdate(ctx context.Context, pull *issues_model.PullRequest, user *user_model.User) (pushAllowed, rebaseAllowed bool, err error) {
	if pull.Flow == issues_model.PullRequestFlowAGit {
		return false, false, nil
	}
	if user == nil {
		return false, false, nil
	}

	// 1. check user push permission on head repository
	pushAllowed, rebaseAllowed, err = isUserAllowedToPushOrForcePushInRepoBranch(ctx, user, pull.HeadRepo, pull.HeadBranch)
	if err != nil {
		return false, false, err
	}

	// 2. check base repository's AllowRebaseUpdate configuration
	// it is a config in base repo but controls the head (fork) repo's "Update" behavior
	if err := pull.LoadBaseRepo(ctx); err != nil {
		return false, false, err
	}
	prBaseUnit, err := pull.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
	if repo_model.IsErrUnitTypeNotExist(err) {
		return false, false, nil // the PR unit is disabled in base repo means no update allowed
	} else if err != nil {
		return false, false, fmt.Errorf("get base repo unit: %v", err)
	}
	rebaseAllowed = rebaseAllowed && prBaseUnit.PullRequestsConfig().AllowRebaseUpdate

	// 3. if the pull creator allows maintainer to edit, we needs to check whether
	// user is a maintainer and inherit pull request creator's permission
	if pull.AllowMaintainerEdit && (!pushAllowed || !rebaseAllowed) {
		baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, pull.BaseRepo, user)
		if err != nil {
			return false, false, err
		}
		userAllowedToMergePR, err := isUserAllowedToMergeInRepoBranch(ctx, pull.BaseRepoID, pull.BaseBranch, baseRepoPerm, user)
		if err != nil {
			return false, false, err
		}
		if userAllowedToMergePR { // if user is maintainer, then it can inherit the poster's push/rebase permission
			if err := pull.LoadIssue(ctx); err != nil {
				return false, false, err
			}
			if err := pull.Issue.LoadPoster(ctx); err != nil {
				return false, false, err
			}
			posterPushAllowed, posterRebaseAllowed, err := isUserAllowedToPushOrForcePushInRepoBranch(ctx, pull.Issue.Poster, pull.HeadRepo, pull.HeadBranch)
			if err != nil {
				return false, false, err
			}
			if !pushAllowed {
				pushAllowed = posterPushAllowed
			}
			if !rebaseAllowed {
				rebaseAllowed = posterRebaseAllowed
			}
		}
	}

	return pushAllowed, rebaseAllowed, nil
}

func syncCommitDivergence(ctx context.Context, pr *issues_model.PullRequest) error {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return err
	}
	divergence, err := gitrepo.GetDivergingCommits(ctx, pr.BaseRepo, pr.BaseBranch, pr.GetGitHeadRefName())
	if err != nil {
		return err
	}
	return pr.UpdateCommitDivergence(ctx, divergence.Ahead, divergence.Behind)
}
