// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"gitea.dev/models/db"
	"gitea.dev/models/migrations/base"
)

func ConvertTopicNameFrom25To50(x db.EngineMigration) error {
	type Topic struct {
		ID          int64  `xorm:"pk autoincr"`
		Name        string `xorm:"UNIQUE VARCHAR(50)"`
		RepoCount   int
		CreatedUnix int64 `xorm:"INDEX created"`
		UpdatedUnix int64 `xorm:"INDEX updated"`
	}

	if err := x.Sync(new(Topic)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.RecreateTable(sess, new(Topic)); err != nil {
		return err
	}

	return sess.Commit()
}
