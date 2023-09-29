// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func RenameRepoIsBareToIsEmpty(x *xorm.Engine) error {
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		IsBare  bool
		IsEmpty bool `xorm:"INDEX"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(Repository)); err != nil {
		return err
	}
	if _, err := sess.Exec("UPDATE repository SET is_empty = is_bare;"); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return err
	}

	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "repository", "is_bare"); err != nil {
		return err
	}

	return sess.Commit()
}
