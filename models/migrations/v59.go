// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"code.gitea.io/gitea/modules/util"
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

	// Updated the comment table
	type Comment struct {
		ID              int64 `xorm:"pk autoincr"`
		Type            int
		PosterID        int64 `xorm:"INDEX"`
		IssueID         int64 `xorm:"INDEX"`
		LabelID         int64
		OldMilestoneID  int64
		MilestoneID     int64
		OldAssigneeID   int64
		AssigneeID      int64
		RemovedAssignee bool
		OldTitle        string
		NewTitle        string

		CommitID        int64
		Line            int64
		Content         string `xorm:"TEXT"`
		RenderedContent string `xorm:"-"`

		CreatedUnix util.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix util.TimeStamp `xorm:"INDEX updated"`

		// Reference issue in commit message
		CommitSHA string `xorm:"VARCHAR(40)"`
	}
	err = x.Sync2(Comment{})
	if err != nil {
		return err
	}

	// Migrate comments
	// First update everything to not have nulls in db
	_, err = x.Where("type = ?", 9).Update(Comment{RemovedAssignee: false})

	allAssignementComments := []Comment{}
	err = x.Where("type = ?", 9).Find(&allAssignementComments)

	for _, comment := range allAssignementComments {
		// Everytime where OldAssigneeID is > 0, the assignement was removed.
		if comment.OldAssigneeID > 0 {
			_, err = x.Id(comment.ID).Update(Comment{RemovedAssignee: true})
		}
	}
}
