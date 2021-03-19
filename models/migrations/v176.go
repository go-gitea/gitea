// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func deleteOrphanedIssueLabels(x *xorm.Engine) error {
	type IssueLabel struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"UNIQUE(s)"`
		LabelID int64 `xorm:"UNIQUE(s)"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := x.Sync2(new(IssueLabel)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := x.In("id", builder.Select("issue_label.id").From("issue_label").
		Join("LEFT", "label", "issue_label.label_id = label.id").
		Where(builder.IsNull{"label.id"})).
		Delete(IssueLabel{}); err != nil {
		return err
	}

	return sess.Commit()
}
