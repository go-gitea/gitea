// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
)

// IssueUser represents an issue-user relation.
type IssueUser struct {
	ID          int64 `xorm:"pk autoincr"`
	UID         int64 `xorm:"INDEX"` // User ID.
	IssueID     int64 `xorm:"INDEX"`
	IsRead      bool
	IsMentioned bool
}

func init() {
	db.RegisterModel(new(IssueUser))
}

// NewIssueUsers inserts an issue related users
func NewIssueUsers(ctx context.Context, repo *repo_model.Repository, issue *Issue) error {
	assignees, err := repo_model.GetRepoAssignees(ctx, repo)
	if err != nil {
		return fmt.Errorf("getAssignees: %w", err)
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

	return db.Insert(ctx, issueUsers)
}

// UpdateIssueUserByRead updates issue-user relation for reading.
func UpdateIssueUserByRead(ctx context.Context, uid, issueID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `issue_user` SET is_read=? WHERE uid=? AND issue_id=?", true, uid, issueID)
	return err
}

// UpdateIssueUsersByMentions updates issue-user pairs by mentioning.
func UpdateIssueUsersByMentions(ctx context.Context, issueID int64, uids []int64) error {
	for _, uid := range uids {
		iu := &IssueUser{
			UID:     uid,
			IssueID: issueID,
		}
		has, err := db.GetEngine(ctx).Get(iu)
		if err != nil {
			return err
		}

		iu.IsMentioned = true
		if has {
			_, err = db.GetEngine(ctx).ID(iu.ID).Cols("is_mentioned").Update(iu)
		} else {
			_, err = db.GetEngine(ctx).Insert(iu)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// GetIssueMentionIDs returns all mentioned user IDs of an issue.
func GetIssueMentionIDs(ctx context.Context, issueID int64) ([]int64, error) {
	var ids []int64
	return ids, db.GetEngine(ctx).Table(IssueUser{}).
		Where("issue_id=?", issueID).
		And("is_mentioned=?", true).
		Select("uid").
		Find(&ids)
}
