// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"context"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func AddActionsVisibility(x *xorm.Engine) error {
	// This migration maybe rerun so that we should check if it has been run
	hasActionsVisibility, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "user", "actions_visibility")
	if err != nil || hasActionsVisibility {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type User struct {
		ActionsVisibility structs.ActionsVisibility `xorm:"NOT NULL DEFAULT 0"`
	}

	if err = x.Sync(&User{}); err != nil {
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
