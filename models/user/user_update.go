// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

func IncrUserRepoNumUnlessArchived(ctx context.Context, userID int64) (bool, error) {
	affected, err := db.GetEngine(ctx).Incr("num_repos").Where(builder.Eq{"id": userID, "is_archived": false}).Update(new(User))
	return affected > 0, err
}

func IsUserArchived(ctx context.Context, userID int64) (bool, error) {
	return db.Exist[User](ctx, builder.Eq{"id": userID, "is_archived": true})
}
