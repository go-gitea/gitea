// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// IssueAssignees saves all issue assignees
type IssueAssignees struct {
	ID         int64 `xorm:"pk autoincr"`
	AssigneeID int64 `xorm:"INDEX"`
	IssueID    int64 `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(IssueAssignees))
}

// LoadAssignees load assignees of this issue.
func (issue *Issue) LoadAssignees() error {
	return issue.loadAssignees(db.GetEngine(db.DefaultContext))
}

// This loads all assignees of an issue
func (issue *Issue) loadAssignees(e db.Engine) (err error) {
	// Reset maybe preexisting assignees
	issue.Assignees = []*user_model.User{}

	err = e.Table("`user`").
		Join("INNER", "issue_assignees", "assignee_id = `user`.id").
		Where("issue_assignees.issue_id = ?", issue.ID).
		Find(&issue.Assignees)

	if err != nil {
		return err
	}

	// Check if we have at least one assignee and if yes put it in as `Assignee`
	if len(issue.Assignees) > 0 {
		issue.Assignee = issue.Assignees[0]
	}

	return
}

// GetAssigneeIDsByIssue returns the IDs of users assigned to an issue
// but skips joining with `user` for performance reasons.
// User permissions must be verified elsewhere if required.
func GetAssigneeIDsByIssue(issueID int64) ([]int64, error) {
	userIDs := make([]int64, 0, 5)
	return userIDs, db.GetEngine(db.DefaultContext).Table("issue_assignees").
		Cols("assignee_id").
		Where("issue_id = ?", issueID).
		Distinct("assignee_id").
		Find(&userIDs)
}

// GetAssigneesByIssue returns everyone assigned to that issue
func GetAssigneesByIssue(issue *Issue) (assignees []*user_model.User, err error) {
	return getAssigneesByIssue(db.GetEngine(db.DefaultContext), issue)
}

func getAssigneesByIssue(e db.Engine, issue *Issue) (assignees []*user_model.User, err error) {
	err = issue.loadAssignees(e)
	if err != nil {
		return assignees, err
	}

	return issue.Assignees, nil
}

// IsUserAssignedToIssue returns true when the user is assigned to the issue
func IsUserAssignedToIssue(issue *Issue, user *user_model.User) (isAssigned bool, err error) {
	return isUserAssignedToIssue(db.GetEngine(db.DefaultContext), issue, user)
}

func isUserAssignedToIssue(e db.Engine, issue *Issue, user *user_model.User) (isAssigned bool, err error) {
	return e.Get(&IssueAssignees{IssueID: issue.ID, AssigneeID: user.ID})
}

// ClearAssigneeByUserID deletes all assignments of an user
func clearAssigneeByUserID(sess db.Engine, userID int64) (err error) {
	_, err = sess.Delete(&IssueAssignees{AssigneeID: userID})
	return
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func (issue *Issue) ToggleAssignee(doer *user_model.User, assigneeID int64) (removed bool, comment *Comment, err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return false, nil, err
	}
	defer committer.Close()

	removed, comment, err = issue.toggleAssignee(ctx, doer, assigneeID, false)
	if err != nil {
		return false, nil, err
	}

	if err := committer.Commit(); err != nil {
		return false, nil, err
	}

	return removed, comment, nil
}

func (issue *Issue) toggleAssignee(ctx context.Context, doer *user_model.User, assigneeID int64, isCreate bool) (removed bool, comment *Comment, err error) {
	sess := db.GetEngine(ctx)
	removed, err = toggleUserAssignee(sess, issue, assigneeID)
	if err != nil {
		return false, nil, fmt.Errorf("UpdateIssueUserByAssignee: %v", err)
	}

	// Repo infos
	if err = issue.loadRepo(ctx); err != nil {
		return false, nil, fmt.Errorf("loadRepo: %v", err)
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
	comment, err = createComment(ctx, opts)
	if err != nil {
		return false, nil, fmt.Errorf("createComment: %v", err)
	}

	// if pull request is in the middle of creation - don't call webhook
	if isCreate {
		return removed, comment, err
	}

	return removed, comment, nil
}

// toggles user assignee state in database
func toggleUserAssignee(e db.Engine, issue *Issue, assigneeID int64) (removed bool, err error) {
	// Check if the user exists
	assignee, err := user_model.GetUserByIDEngine(e, assigneeID)
	if err != nil {
		return false, err
	}

	// Check if the submitted user is already assigned, if yes delete him otherwise add him
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

// MakeIDsFromAPIAssigneesToAdd returns an array with all assignee IDs
func MakeIDsFromAPIAssigneesToAdd(oneAssignee string, multipleAssignees []string) (assigneeIDs []int64, err error) {
	var requestAssignees []string

	// Keeping the old assigning method for compatibility reasons
	if oneAssignee != "" && !util.IsStringInSlice(oneAssignee, multipleAssignees) {
		requestAssignees = append(requestAssignees, oneAssignee)
	}

	// Prevent empty assignees
	if len(multipleAssignees) > 0 && multipleAssignees[0] != "" {
		requestAssignees = append(requestAssignees, multipleAssignees...)
	}

	// Get the IDs of all assignees
	assigneeIDs, err = user_model.GetUserIDsByNames(requestAssignees, false)

	return
}
