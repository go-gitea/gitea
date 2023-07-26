// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// IssueAssignees saves all issue assignees
type IssueAssignees struct {
	ID                 int64            `xorm:"pk autoincr"`
	AssigneeID         int64            `xorm:"INDEX"`
	Assignee           *user_model.User `xorm:"-"`
	OriginalAssignee   string
	OriginalAssigneeID int64 `xorm:"index"`
	IssueID            int64 `xorm:"INDEX"`
}

// GetUserID ExternalUserRemappable interface
func (ia *IssueAssignees) GetUserID() int64 { return ia.AssigneeID }

// GetExternalName ExternalUserRemappable interface
func (ia *IssueAssignees) GetExternalName() string { return ia.OriginalAssignee }

// GetExternalID ExternalUserRemappable interface
func (ia *IssueAssignees) GetExternalID() int64 { return ia.OriginalAssigneeID }

// RemapExternalUser ExternalUserRemappable interface
func (ia *IssueAssignees) RemapExternalUser(externalName string, externalID, userID int64) error {
	ia.OriginalAssignee = externalName
	ia.OriginalAssigneeID = externalID
	ia.AssigneeID = userID
	return nil
}

func init() {
	db.RegisterModel(new(IssueAssignees))
}

// LoadAssignees load assignees of this issue.
func (issue *Issue) LoadAssignees(ctx context.Context) error {
	// Reset maybe preexisting assignees
	issue.Assignees = []*IssueAssignees{}
	issue.Assignee = nil

	if err := db.GetEngine(ctx).Table("`user`").
		Where("issue_assignees.issue_id = ?", issue.ID).
		Find(&issue.Assignees); err != nil {
		return err
	}

	var assigneeIDs []int64
	for _, assignee := range issue.Assignees {
		assigneeIDs = append(assigneeIDs, assignee.ID)
	}
	assigneesMap := make(map[int64]*user_model.User)
	if err := db.GetEngine(ctx).In("id", assigneeIDs).Find(&assigneesMap); err != nil {
		return err
	}
	for _, assignee := range issue.Assignees {
		if assigneesMap[assignee.ID] != nil {
			assignee.Assignee = assigneesMap[assignee.ID]
		}
	}

	// Check if we have at least one assignee and if yes put it in as `Assignee`
	if len(issue.Assignees) > 0 {
		issue.Assignee = issue.Assignees[0]
	}
	return nil
}

// GetAssigneeIDsByIssue returns the IDs of users assigned to an issue
// but skips joining with `user` for performance reasons.
// User permissions must be verified elsewhere if required.
func GetAssigneeIDsByIssue(ctx context.Context, issueID int64) ([]int64, error) {
	userIDs := make([]int64, 0, 5)
	return userIDs, db.GetEngine(ctx).
		Table("issue_assignees").
		Cols("assignee_id").
		Where("issue_id = ?", issueID).
		Distinct("assignee_id").
		Find(&userIDs)
}

// IsUserAssignedToIssue returns true when the user is assigned to the issue
func IsUserAssignedToIssue(ctx context.Context, issue *Issue, user *user_model.User) (isAssigned bool, err error) {
	return db.GetByBean(ctx, &IssueAssignees{IssueID: issue.ID, AssigneeID: user.ID})
}

// ToggleIssueAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleIssueAssignee(ctx context.Context, issue *Issue, doer *user_model.User, assigneeID int64) (removed bool, comment *Comment, err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return false, nil, err
	}
	defer committer.Close()

	removed, comment, err = toggleIssueAssignee(ctx, issue, doer, assigneeID, false)
	if err != nil {
		return false, nil, err
	}

	if err := committer.Commit(); err != nil {
		return false, nil, err
	}

	return removed, comment, nil
}

func toggleIssueAssignee(ctx context.Context, issue *Issue, doer *user_model.User, assigneeID int64, isCreate bool) (removed bool, comment *Comment, err error) {
	removed, err = toggleUserAssignee(ctx, issue, assigneeID)
	if err != nil {
		return false, nil, fmt.Errorf("UpdateIssueUserByAssignee: %w", err)
	}

	// Repo infos
	if err = issue.LoadRepo(ctx); err != nil {
		return false, nil, fmt.Errorf("loadRepo: %w", err)
	}

	opts := &CreateCommentOptions{
		Type:            CommentTypeAssignees,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: removed,
		AssigneeID:      assigneeID,
	}
	// Comment
	comment, err = CreateComment(ctx, opts)
	if err != nil {
		return false, nil, fmt.Errorf("createComment: %w", err)
	}

	// if pull request is in the middle of creation - don't call webhook
	if isCreate {
		return removed, comment, err
	}

	return removed, comment, nil
}

// toggles user assignee state in database
func toggleUserAssignee(ctx context.Context, issue *Issue, assigneeID int64) (removed bool, err error) {
	// Check if the user exists
	_, err = user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return false, err
	}

	// Check if the submitted user is already assigned, if yes delete him otherwise add him
	found := false
	i := 0
	for ; i < len(issue.Assignees); i++ {
		if issue.Assignees[i].ID == assigneeID {
			found = true
			break
		}
	}

	assigneeIn := IssueAssignees{AssigneeID: assigneeID, IssueID: issue.ID}
	if found {
		issue.Assignees = append(issue.Assignees[:i], issue.Assignees[i+1:]...)
		_, err = db.DeleteByBean(ctx, &assigneeIn)
		if err != nil {
			return found, err
		}
	} else {
		issue.Assignees = append(issue.Assignees, &assigneeIn)
		if err = db.Insert(ctx, &assigneeIn); err != nil {
			return found, err
		}
	}

	return found, nil
}

// MakeIDsFromAPIAssigneesToAdd returns an array with all assignee IDs
func MakeIDsFromAPIAssigneesToAdd(ctx context.Context, oneAssignee string, multipleAssignees []string) (assigneeIDs []int64, err error) {
	var requestAssignees []string

	// Keeping the old assigning method for compatibility reasons
	if oneAssignee != "" && !util.SliceContainsString(multipleAssignees, oneAssignee) {
		requestAssignees = append(requestAssignees, oneAssignee)
	}

	// Prevent empty assignees
	if len(multipleAssignees) > 0 && multipleAssignees[0] != "" {
		requestAssignees = append(requestAssignees, multipleAssignees...)
	}

	// Get the IDs of all assignees
	assigneeIDs, err = user_model.GetUserIDsByNames(ctx, requestAssignees, false)

	return assigneeIDs, err
}
