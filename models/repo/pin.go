// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

type Pin struct {
	ID          int64              `xorm:"pk autoincr"`
	UID         int64              `xorm:"UNIQUE(s)"`
	RepoID      int64              `xorm:"UNIQUE(s)"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

// TableName sets the table name for the pin struct
func (s *Pin) TableName() string {
	return "repository_pin"
}

func init() {
	db.RegisterModel(new(Pin))
}

func IsPinned(ctx context.Context, userID, repoID int64) bool {
	exists, _ := db.GetEngine(ctx).Exist(&Pin{UID: userID, RepoID: repoID})

	return exists
}

func PinRepo(ctx context.Context, doer *user_model.User, repo *Repository, pin bool) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		pinned := IsPinned(ctx, doer.ID, repo.ID)

		if pin {
			// Already pinned, nothing to do
			if pinned {
				return nil
			}

			if err := db.Insert(ctx, &Pin{UID: doer.ID, RepoID: repo.ID}); err != nil {
				return err
			}
		} else {
			// Not pinned, nothing to do
			if !pinned {
				return nil
			}

			if _, err := db.DeleteByBean(ctx, &Pin{UID: doer.ID, RepoID: repo.ID}); err != nil {
				return err
			}
		}

		return nil
	})
}
