// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "gitea.dev/models/db"

func AddIndexOnRepositoryAndComment(x db.EngineMigration) error {
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		OwnerID int64 `xorm:"index"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	type Comment struct {
		ID       int64 `xorm:"pk autoincr"`
		Type     int   `xorm:"index"`
		ReviewID int64 `xorm:"index"`
	}

	return x.Sync(new(Comment))
}
