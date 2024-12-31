// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
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
func (issue *Issue) LoadAssignees(ctx context.Context) (err error) {
	if issue.isAssigneeLoaded || len(issue.Assignees) > 0 {
		return nil
	}

	// Reset maybe preexisting assignees
	issue.Assignees = []*user_model.User{}
	issue.Assignee = nil

	if err = db.GetEngine(ctx).Table("`user`").
		Join("INNER", "issue_assignees", "assignee_id = `user`.id").
		Where("issue_assignees.issue_id = ?", issue.ID).
		Find(&issue.Assignees); err != nil {
		return err
	}

	issue.isAssigneeLoaded = true
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
	return db.Exist[IssueAssignees](ctx, builder.Eq{"assignee_id": user.ID, "issue_id": issue.ID})
}

type AssignedIssuesOptions struct {
	db.ListOptions
	AssigneeID  int64
	RepoOwnerID int64
}

func (opts *AssignedIssuesOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.AssigneeID != 0 {
		cond = cond.And(builder.In("issue.id", builder.Select("issue_id").From("issue_assignees").Where(builder.Eq{"assignee_id": opts.AssigneeID})))
	}
	if opts.RepoOwnerID != 0 {
		cond = cond.And(builder.In("issue.repo_id", builder.Select("id").From("repository").Where(builder.Eq{"owner_id": opts.RepoOwnerID})))
	}
	return cond
}

func GetAssignedIssues(ctx context.Context, opts *AssignedIssuesOptions) ([]*Issue, int64, error) {
	return db.FindAndCount[Issue](ctx, opts)
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
	assignee, err := user_model.GetUserByID(ctx, assigneeID)
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
		issue.Assignees = append(issue.Assignees, assignee)
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
