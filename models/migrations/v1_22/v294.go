// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"fmt"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// AddUniqueIndexForProjectIssue adds unique indexes for project issue table
func AddUniqueIndexForProjectIssue(x *xorm.Engine) error {
	// remove possible duplicated records in table project_issue
	type result struct {
		IssueID   int64
		ProjectID int64
		Cnt       int
	}
	var results []result
	if err := x.Select("issue_id, project_id, count(*) as cnt").
		Table("project_issue").
		GroupBy("issue_id, project_id").
		Having("count(*) > 1").
		Find(&results); err != nil {
		return err
	}
	for _, r := range results {
		if x.Dialect().URI().DBType == schemas.MSSQL {
			if _, err := x.Exec(fmt.Sprintf("delete from project_issue where id in (SELECT top %d id FROM project_issue WHERE issue_id = ? and project_id = ?)", r.Cnt-1), r.IssueID, r.ProjectID); err != nil {
				return err
			}
		} else {
			var ids []int64
			if err := x.SQL("SELECT id FROM project_issue WHERE issue_id = ? and project_id = ? limit ?", r.IssueID, r.ProjectID, r.Cnt-1).Find(&ids); err != nil {
				return err
			}
			if _, err := x.Table("project_issue").In("id", ids).Delete(); err != nil {
				return err
			}
		}
	}

	// add unique index for project_issue table
	type ProjectIssue struct { //revive:disable-line:exported
		ID        int64 `xorm:"pk autoincr"`
		IssueID   int64 `xorm:"INDEX unique(s)"`
		ProjectID int64 `xorm:"INDEX unique(s)"`
	}

	return x.Sync(new(ProjectIssue))
}
