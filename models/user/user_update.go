// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

func IncrUserRepoNum(ctx context.Context, userID int64) error {
	_, err := db.GetEngine(ctx).Incr("num_repos").ID(userID).Update(new(User))
	return err
}

func IsUserArchived(ctx context.Context, userID int64) (bool, error) {
	return db.Exist[User](ctx, builder.Eq{"id": userID, "is_archived": true})
}
