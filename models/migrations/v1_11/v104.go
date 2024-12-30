// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func RemoveLabelUneededCols(x *xorm.Engine) error {
	// Make sure the columns exist before dropping them
	type Label struct {
		QueryString string
		IsSelected  bool
	}
	if err := x.Sync(new(Label)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "label", "query_string"); err != nil {
		return err
	}
	if err := base.DropTableColumns(sess, "label", "is_selected"); err != nil {
		return err
	}
	return sess.Commit()
}
