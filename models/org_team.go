// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

func ReconsiderRepoIssuesAssignee(ctx context.Context, repo *repo_model.Repository, user *user_model.User) error {
	if canAssigned, err := access_model.CanBeAssigned(ctx, user, repo, true); err != nil || canAssigned {
		return err
	}

	if _, err := db.GetEngine(ctx).Where(builder.Eq{"assignee_id": user.ID}).
		In("issue_id", builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repo.ID})).
		Delete(&issues_model.IssueAssignees{}); err != nil {
		return fmt.Errorf("Could not delete assignee[%d] %w", user.ID, err)
	}
	return nil
}

func ReconsiderWatches(ctx context.Context, repo *repo_model.Repository, user *user_model.User) error {
	if has, err := access_model.HasAnyUnitAccess(ctx, user.ID, repo); err != nil || has {
		return err
	}
	if err := repo_model.WatchRepo(ctx, user, repo, false); err != nil {
		return err
	}

	// Remove all IssueWatches a user has subscribed to in the repository
	return issues_model.RemoveIssueWatchersByRepoID(ctx, user.ID, repo.ID)
}
