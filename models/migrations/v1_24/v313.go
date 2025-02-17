// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func MovePinOrderToTableIssuePin(x *xorm.Engine) error {
	type IssuePin struct {
		ID       int64 `xorm:"pk autoincr"`
		RepoID   int64 `xorm:"UNIQUE(s) NOT NULL"`
		IssueID  int64 `xorm:"UNIQUE(s) NOT NULL"`
		IsPull   bool  `xorm:"NOT NULL"`
		PinOrder int   `xorm:"DEFAULT 0"`
	}

	if err := x.Sync(new(IssuePin)); err != nil {
		return err
	}

	if _, err := x.Exec("INSERT INTO issue_pin (repo_id, issue_id, is_pull, pin_order) SELECT repo_id, id, is_pull, pin_order FROM issue WHERE pin_order > 0"); err != nil {
		return err
	}
	sess := x.NewSession()
	defer sess.Close()
	return base.DropTableColumns(sess, "issue", "pin_order")
}
