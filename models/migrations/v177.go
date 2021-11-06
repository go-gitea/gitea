// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

// deleteOrphanedIssueLabels looks through the database for issue_labels where the label no longer exists and deletes them.
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

	if err := sess.Sync2(new(IssueLabel)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := sess.Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
		SELECT ill.id FROM (
			SELECT il.id
			FROM issue_label AS il
				LEFT JOIN label ON il.label_id = label.id
			WHERE
				label.id IS NULL
		) AS ill)`); err != nil {
		return err
	}

	return sess.Commit()
}
