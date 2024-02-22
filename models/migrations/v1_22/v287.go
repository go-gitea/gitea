// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/structs"

	"code.gitea.io/gitea/models/user"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func AddActionsVisibility(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.
		Where(builder.Eq{"keep_activity_private": 1}).
		Cols("actions_visibility").
		Update(user.User{ActionsVisibility: structs.ActionsVisibilityNone}); err != nil {
		return err
	}

	if err := base.DropTableColumns(sess, "user", "keep_activity_private"); err != nil {
		return err
	}

	return sess.Commit()
}
