// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"gitea.dev/models/db"
	"gitea.dev/models/migrations/base"
)

func DropCustomLabelsColumnOfActionRunner(x db.EngineMigration) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	// drop "custom_labels" cols
	if err := base.DropTableColumns(sess, "action_runner", "custom_labels"); err != nil {
		return err
	}

	return sess.Commit()
}
