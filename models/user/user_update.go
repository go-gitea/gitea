// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

func IncrUserRepoNum(ctx context.Context, userID int64) error {
	_, err := db.GetEngine(ctx).Incr("num_repos").ID(userID).Update(new(User))
	return err
}
