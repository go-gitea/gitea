// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

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
