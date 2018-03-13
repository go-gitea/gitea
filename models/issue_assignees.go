// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/sdk/gitea"

	"github.com/go-xorm/xorm"
)

// This loads all assignees of an issue
func (issue *Issue) loadAssignees(e Engine) (err error) {
	var assigneeIDs []IssueAssignees

	err = e.Where("issue_id = ?", issue.ID).Find(&assigneeIDs)
	if err != nil {
		return
	}

	for _, assignee := range assigneeIDs {
		user, err := getUserByID(e, assignee.AssigneeID)
		if err != nil {
			user = NewGhostUser()
			if !IsErrUserNotExist(err) {
				return err
			}
		}
		issue.Assignees = append(issue.Assignees, user)
	}

	// Check if we have at least one assignee and if yes put it in as `Assignee`
	if len(issue.Assignees) > 0 {
		issue.Assignee = issue.Assignees[0]
	}

	return
}

// GetAssigneesByIssue returns everyone assigned to that issue
func GetAssigneesByIssue(issue *Issue) (assignees []*User, err error) {
	err = issue.loadAssignees(x)
	if err != nil {
		return assignees, err
	}

	return issue.Assignees, nil
}

// IsUserAssignedToIssue returns true when the user is assigned to the issue
func IsUserAssignedToIssue(issue *Issue, user *User) (isAssigned bool, err error) {
	assignees := IssueAssignees{AssigneeID:user.ID, IssueID:issue.ID}
	isAssigned, err = x.Get(&assignees)
	return
}

// ClearAssigneesByIssue deletes all assignees for one issue
func ClearAssigneesByIssue(issue *Issue) (err error) {
	_, err = x.Delete(IssueAssignees{IssueID: issue.ID})
	if err != nil {
		return err
	}
	return nil
}

// MakeAssigneeList concats a string with all names of the assignees. Useful for logs.
func MakeAssigneeList(issue Issue) (AssigneeList string, err error) {
	err = issue.loadAssignees(x)
	if err != nil {
		return "", err
	}

	for in, assignee := range issue.Assignees {
		AssigneeList += assignee.Name

		if len(issue.Assignees) > (in + 1) {
			AssigneeList += ", "
		}
	}
	return
}

// UpdateAssignee deletes or adds an assignee to an issue
func UpdateAssignee(issue *Issue, doer *User, assigneeID int64) (err error) {
	return issue.ChangeAssignee(doer, assigneeID)
}

// ChangeAssignee changes the Assignee of this issue.
func (issue *Issue) ChangeAssignee(doer *User, assigneeID int64) (err error) {
	sess := x.NewSession()
	defer sess.Close()

	return issue.changeAssignee(sess, doer, assigneeID)
}

func (issue *Issue) changeAssignee(sess *xorm.Session, doer *User, assigneeID int64) (err error) {

	// Update the assignee
	removed, err := updateIssueAssignee(sess, issue, assigneeID)
	if err != nil {
		return fmt.Errorf("UpdateIssueUserByAssignee: %v", err)
	}

	// Repo infos
	if err = issue.loadRepo(sess); err != nil {
		return fmt.Errorf("loadRepo: %v", err)
	}

	// Comment
	if _, err = createAssigneeComment(sess, doer, issue.Repo, issue, assigneeID, removed); err != nil {
		return fmt.Errorf("createAssigneeComment: %v", err)
	}

	if issue.IsPull {
		issue.PullRequest = &PullRequest{Issue: issue}
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: issue.PullRequest.APIFormat(),
			Repository:  issue.Repo.APIFormat(AccessModeNone),
			Sender:      doer.APIFormat(),
		}
		if removed {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		if err := PrepareWebhooks(issue.Repo, HookEventPullRequest, apiPullRequest); err != nil {
			log.Error(4, "PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return nil
		}
	}
	go HookQueue.Add(issue.RepoID)
	return nil
}

func AddAssigneeByName(assigneeName string, issue *Issue, doer *User) (err error) {
	assignee, err := GetUserByName(assigneeName)
	if err != nil {
		return
	}

	// update
	return UpdateAssignee(issue, doer, assignee.ID)
}
