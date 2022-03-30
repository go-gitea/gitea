// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
)

// CheckPullProtection check if the pull mergable based on all conditions (branch protection, merge options, ...)
func CheckPullProtection(ctx context.Context, doer *user_model.User, perm *models.Permission, pr *models.PullRequest, manuallMerge, force bool) error {
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
		// dont check rules to "auto merge", doer is goint to mark this pull as merged manually
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

	if _, err := IsSignedIfRequired(pr, doer); err != nil {
		return err
	}

	if noDeps, err := models.IssueNoDependenciesLeft(pr.Issue); err != nil {
		return err
	} else if !noDeps {
		return ErrDependenciesLeft{}
	}

	return nil
}
