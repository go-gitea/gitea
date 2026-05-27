// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"

	"xorm.io/builder"
)

func GetGroupRepos(ctx context.Context, groupID int64, doer *user_model.User) ([]*repo_model.Repository, error) {
	sess := db.GetEngine(ctx)
	repos := make([]*repo_model.Repository, 0)
	return repos, sess.Table("repository").
		Where("group_id = ?", groupID).
		And(builder.In("id", repo_model.AccessibleRepoIDsQuery(doer))).
		OrderBy("group_sort_order").
		Find(&repos)
}
