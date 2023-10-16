// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func RemoveActionRunnerTokenUnneededCol(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "action_runner_token", "deleted"); err != nil {
		return err
	}
	return sess.Commit()
}
