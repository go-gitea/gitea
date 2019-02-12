// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

// IssueUser represents an issue-user relation.
type IssueUser struct {
	ID          int64 `xorm:"pk autoincr"`
	UID         int64 `xorm:"INDEX"` // User ID.
	IssueID     int64
	IsRead      bool
	IsMentioned bool
}

func newIssueUsers(e Engine, repo *Repository, issue *Issue) error {
	assignees, err := repo.getAssignees(e)
	if err != nil {
		return fmt.Errorf("getAssignees: %v", err)
	}

	// Poster can be anyone, append later if not one of assignees.
	isPosterAssignee := false

	// Leave a seat for poster itself to append later, but if poster is one of assignee
	// and just waste 1 unit is cheaper than re-allocate memory once.
	issueUsers := make([]*IssueUser, 0, len(assignees)+1)
	for _, assignee := range assignees {
		issueUsers = append(issueUsers, &IssueUser{
			IssueID: issue.ID,
			UID:     assignee.ID,
		})
		isPosterAssignee = isPosterAssignee || assignee.ID == issue.PosterID
	}
	if !isPosterAssignee {
		issueUsers = append(issueUsers, &IssueUser{
			IssueID: issue.ID,
			UID:     issue.PosterID,
		})
	}

	if _, err = e.Insert(issueUsers); err != nil {
		return err
	}
	return nil
}

func updateIssueAssignee(e *xorm.Session, issue *Issue, assigneeID int64) (removed bool, err error) {

	// Check if the user exists
	assignee, err := getUserByID(e, assigneeID)
	if err != nil {
		return false, err
	}

	// Check if the submitted user is already assigne, if yes delete him otherwise add him
	var i int
	for i = 0; i < len(issue.Assignees); i++ {
		if issue.Assignees[i].ID == assigneeID {
			break
		}
	}

	assigneeIn := IssueAssignees{AssigneeID: assigneeID, IssueID: issue.ID}

	toBeDeleted := i < len(issue.Assignees)
	if toBeDeleted {
		issue.Assignees = append(issue.Assignees[:i], issue.Assignees[i:]...)
		_, err = e.Delete(assigneeIn)
		if err != nil {
			return toBeDeleted, err
		}
	} else {
		issue.Assignees = append(issue.Assignees, assignee)
		_, err = e.Insert(assigneeIn)
		if err != nil {
			return toBeDeleted, err
		}
	}

	return toBeDeleted, nil
}

// UpdateIssueUserByRead updates issue-user relation for reading.
func UpdateIssueUserByRead(uid, issueID int64) error {
	_, err := x.Exec("UPDATE `issue_user` SET is_read=? WHERE uid=? AND issue_id=?", true, uid, issueID)
	return err
}

// UpdateIssueUsersByMentions updates issue-user pairs by mentioning.
func UpdateIssueUsersByMentions(e Engine, issueID int64, uids []int64) error {
	for _, uid := range uids {
		iu := &IssueUser{
			UID:     uid,
			IssueID: issueID,
		}
		has, err := e.Get(iu)
		if err != nil {
			return err
		}

		iu.IsMentioned = true
		if has {
			_, err = e.ID(iu.ID).Cols("is_mentioned").Update(iu)
		} else {
			_, err = e.Insert(iu)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
