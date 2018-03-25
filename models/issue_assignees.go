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

// IssueAssignees saves all issue assignees
type IssueAssignees struct {
	ID         int64 `xorm:"pk autoincr"`
	AssigneeID int64 `xorm:"INDEX"`
	IssueID    int64 `xorm:"INDEX"`
}

// This loads all assignees of an issue
func (issue *Issue) loadAssignees(e Engine) (err error) {
	// Reset maybe preexisting assignees
	issue.Assignees = []*User{}

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
	isAssigned, err = x.Exist(&IssueAssignees{IssueID:issue.ID, AssigneeID: user.ID})
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

// Deletes all assignees who aren't passed via the "assignees" array
func deleteNotPassedAssignee(issue *Issue, doer *User, assigees []*User) (err error) {
	var found bool

	for _, assignee := range issue.Assignees {

		found = false
		for _, alreadyAssignee := range assigees {
			if assignee.ID == alreadyAssignee.ID {
				found = true
				break
			}
		}

		if !found {
			// This function also does comments and hooks, which is why we call it seperatly instead of directly removing the assignees here
			if err := UpdateAssignee(issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// MakeAssigneeList concats a string with all names of the assignees. Useful for logs.
func MakeAssigneeList(issue *Issue) (AssigneeList string, err error) {
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


// ClearAssigneeByUserID deletes all assignments of an user
func clearAssigneeByUserID(sess *xorm.Session, userID int64) (err error) {
	_, err = sess.Delete(&IssueAssignees{AssigneeID: userID})
	return
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't aleady assigned to the issue
func AddAssigneeIfNotAssigned(issue *Issue, doer *User, assigneeID int64) (err error) {
	// Check if the user is already assigned
	isAssigned, err := IsUserAssignedToIssue(issue, &User{ID: assigneeID})
	if err != nil {
		return err
	}

	if !isAssigned {
		return issue.ChangeAssignee(doer, assigneeID)
	}
	return nil
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

// UpdateAPIAssignee is a helper function to add or delete one or multiple issue assignee(s)
// Deleting is done the Github way (quote from their api documentation):
// https://developer.github.com/v3/issues/#edit-an-issue
// "assignees" (array): Logins for Users to assign to this issue.
// Pass one or more user logins to replace the set of assignees on this Issue.
// Send an empty array ([]) to clear all assignees from the Issue.
func UpdateAPIAssignee(issue *Issue, oneAssignee string, multipleAssignees []string, doer *User) (err error) {
	var allNewAssignees []*User

	// Keep the old assignee thingy for compatibility reasons
	if oneAssignee != "" {
		// Prevent double adding assignees
		var isDouble bool
		for _, assignee := range multipleAssignees {
			if assignee == oneAssignee {
				isDouble = true
			}
		}

		if !isDouble {
			multipleAssignees = append(multipleAssignees, oneAssignee)
		}
	}

	// Loop through all assignees to add them
	for _, assigneeName := range multipleAssignees {
		assignee, err := GetUserByName(assigneeName)
		if err != nil {
			return err
		}

		allNewAssignees = append(allNewAssignees, assignee)
	}

	// Delete all old assignees not passed
	if err = deleteNotPassedAssignee(issue, doer, allNewAssignees); err != nil {
		return err
	}

	// Add all new assignees
	// Update the assignee. The function will check if the user exists, is already
	// assigned (which he shouldn't as we deleted all assignees before) and
	// has access to the repo.
	for _, assignee := range allNewAssignees {
		// Extra method to prevent double adding (which would result in removing)
		err = AddAssigneeIfNotAssigned(issue, doer, assignee.ID)
		if err != nil {
			return err
		}
	}

	return
}

// MakeIDsFromAPIAssigneesToAdd returns an array with all assignee IDs
func MakeIDsFromAPIAssigneesToAdd(oneAssignee string, multipleAssignees []string) (assigneeIDs []int64, err error) {

	// Keeping the old assigning method for compatibility reasons
	if len(oneAssignee) > 0 {

		// Prevent double adding assignees
		var isDouble bool
		for _, assignee := range multipleAssignees {
			if assignee == oneAssignee {
				isDouble = true
			}
		}

		if !isDouble {
			multipleAssignees = append(multipleAssignees, oneAssignee)
		}
	}

	// Loop through the assignees
	if len(multipleAssignees) > 0 {
		for _, assigneeName := range multipleAssignees {
			user, err := GetUserByName(assigneeName)
			if err != nil {
				return nil, err
			}

			assigneeIDs = append(assigneeIDs, user.ID)
		}
	}
	return
}
