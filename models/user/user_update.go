// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/db"
)

// IncrUserRepoNum increments a user's repository count if it is below limit.
// A negative limit means no limit.
func IncrUserRepoNum(ctx context.Context, userID int64, limit int) (bool, error) {
	sess := db.GetEngine(ctx).Incr("num_repos").ID(userID)
	if limit >= 0 {
		sess.Where("num_repos < ?", limit)
	}

	updated, err := sess.Update(new(User))
	return updated > 0, err
}
