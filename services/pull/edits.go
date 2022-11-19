// Copyright 2022 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"errors"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
)

var ErrUserHasNoPermissionForAction = errors.New("user not allowed to do this action")

// SetAllowEdits allow edits from maintainers to PRs
func SetAllowEdits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, allow bool) error {
	if doer == nil || !pr.Issue.IsPoster(doer.ID) {
		return ErrUserHasNoPermissionForAction
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}

	permission, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		return err
	}

	if !permission.CanWrite(unit_model.TypeCode) {
		return ErrUserHasNoPermissionForAction
	}

	pr.AllowMaintainerEdit = allow
	return issues_model.UpdateAllowEdits(ctx, pr)
}
