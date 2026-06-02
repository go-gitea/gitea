// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/log"

	"xorm.io/builder"
)

func SetIsArchivedToFalse(x db.EngineMigration) error {
	type Repository struct {
		IsArchived bool `xorm:"INDEX"`
	}
	count, err := x.Where(builder.IsNull{"is_archived"}).Cols("is_archived").Update(&Repository{
		IsArchived: false,
	})
	if err == nil {
		log.Debug("Updated %d repositories with is_archived IS NULL", count)
	}
	return err
}
