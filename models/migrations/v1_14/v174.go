// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"fmt"

	"gitea.dev/models/db"
)

func AddRepoTransfer(x db.EngineMigration) error {
	type RepoTransfer struct {
		ID          int64 `xorm:"pk autoincr"`
		DoerID      int64
		RecipientID int64
		RepoID      int64
		TeamIDs     []int64
		CreatedUnix int64 `xorm:"INDEX NOT NULL created"`
		UpdatedUnix int64 `xorm:"INDEX NOT NULL updated"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(RepoTransfer)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}

	return sess.Commit()
}
