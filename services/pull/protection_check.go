// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

type ErrIsClosed struct{}

func IsErrIsClosed(err error) bool {
	_, ok := err.(ErrIsClosed)
	return ok
}

func (err ErrIsClosed) Error() string {
	return "pull is cosed"
}

type ErrUserNotAllowedToMerge struct{}

func IsErrUserNotAllowedToMerge(err error) bool {
	_, ok := err.(ErrUserNotAllowedToMerge)
	return ok
}

func (err ErrUserNotAllowedToMerge) Error() string {
	return "user not allowed to merge"
}

type ErrHasMerged struct{}

func IsErrHasMerged(err error) bool {
	_, ok := err.(ErrHasMerged)
	return ok
}

func (err ErrHasMerged) Error() string {
	return "has already been merged"
}

type ErrIsWorkInProgress struct{}

func IsErrIsWorkInProgress(err error) bool {
	_, ok := err.(ErrIsWorkInProgress)
	return ok
}

func (err ErrIsWorkInProgress) Error() string {
	return "work in progress PRs cannot be merged"
}

type ErrNotMergableState struct{}

func IsErrNotMergableState(err error) bool {
	_, ok := err.(ErrNotMergableState)
	return ok
}

func (err ErrNotMergableState) Error() string {
	return "not in mergeable state"
}

type ErrDependenciesLeft struct{}

func IsErrDependenciesLeft(err error) bool {
	_, ok := err.(ErrDependenciesLeft)
	return ok
}

func (err ErrDependenciesLeft) Error() string {
	return "is blocked by an open dependency"
}

// CheckPullMergable check if the pull mergable based on all conditions (branch protection, merge options, ...)
func CheckPullMergable(ctx context.Context, doer *user_model.User, perm *models.Permission, pr *models.PullRequest, manuallMerge, force bool) error {
	if pr.HasMerged {
		return ErrHasMerged{}
	}

	if err := pr.LoadIssue(); err != nil {
		return err
	} else if pr.Issue.IsClosed {
		return ErrIsClosed{}
	}

	if allowedMerge, err := IsUserAllowedToMerge(pr, *perm, doer); err != nil {
		return err
	} else if !allowedMerge {
		return ErrUserNotAllowedToMerge{}
	}

	if manuallMerge {
		// don't check rules to "auto merge", doer is going to mark this pull as merged manually
		return nil
	}

	if pr.IsWorkInProgress() {
		return ErrIsWorkInProgress{}
	}

	if !pr.CanAutoMerge() {
		return ErrNotMergableState{}
	}

	if err := CheckPRReadyToMerge(pr, false); err != nil {
		if models.IsErrNotAllowedToMerge(err) {
			if force {
				if isRepoAdmin, err := models.IsUserRepoAdmin(pr.BaseRepo, doer); err != nil {
					return err
				} else if !isRepoAdmin {
					return ErrUserNotAllowedToMerge{}
				}
			}
		} else {
			return err
		}
	}

	if _, err := isSignedIfRequired(pr, doer); err != nil {
		return err
	}

	if noDeps, err := models.IssueNoDependenciesLeft(pr.Issue); err != nil {
		return err
	} else if !noDeps {
		return ErrDependenciesLeft{}
	}

	return nil
}

// isSignedIfRequired check if merge will be signed if required
func isSignedIfRequired(pr *models.PullRequest, doer *user_model.User) (bool, error) {
	if err := pr.LoadProtectedBranch(); err != nil {
		return false, err
	}

	if pr.ProtectedBranch == nil || !pr.ProtectedBranch.RequireSignedCommits {
		return true, nil
	}

	sign, _, _, err := asymkey_service.SignMerge(pr, doer, pr.BaseRepo.RepoPath(), pr.BaseBranch, pr.GetGitRefName())

	return sign, err
}
