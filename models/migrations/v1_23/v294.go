// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

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
		Where("cnt > 1").
		Find(&results); err != nil {
		return err
	}
	for _, r := range results {
		if _, err := x.Exec("DELETE FROM project_issue WHERE issue_id = ? AND project_id = ? ORDER BY id DESC LIMIT ?", r.IssueID, r.ProjectID, r.Cnt-1); err != nil {
			return err
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
