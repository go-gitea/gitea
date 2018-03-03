// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"github.com/go-xorm/xorm"
)

func addMultipleAssignees(x *xorm.Engine) error {

	allIssues := []models.Issue{}
	err := x.Find(&allIssues)
	if err != nil {
		return err
	}

	// Create the table
	type IssueAssignees struct {
		ID         int64 `xorm:"pk autoincr"`
		AssigneeID int64 `xorm:"INDEX"`
		IssueID    int64 `xorm:"INDEX"`
	}
	err = x.Sync2(IssueAssignees{})
	if err != nil {
		return err
	}

	// Range over all issues and insert a new entry for each issue/assignee
	for _, issue := range allIssues {
		if issue.AssigneeID != 0 {
			_, err := x.Insert(IssueAssignees{IssueID: issue.ID, AssigneeID: issue.AssigneeID})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
