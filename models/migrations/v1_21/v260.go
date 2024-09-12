// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func DropCustomLabelsColumnOfActionRunner(x *xorm.Engine) error {
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
