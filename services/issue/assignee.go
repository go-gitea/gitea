// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	notify_service "code.gitea.io/gitea/services/notify"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assignees []*user_model.User) (err error) {
	var found bool
	oriAssignees := make([]*user_model.User, len(issue.Assignees))
	_ = copy(oriAssignees, issue.Assignees)

	for _, assignee := range oriAssignees {
		found = false
		for _, alreadyAssignee := range assignees {
			if assignee.ID == alreadyAssignee.ID {
				found = true
				break
			}
		}

		if !found {
			// This function also does comments and hooks, which is why we call it separately instead of directly removing the assignees here
			if _, _, err := ToggleAssigneeWithNotify(ctx, issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleAssigneeWithNoNotify changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssigneeWithNotify(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64) (removed bool, comment *issues_model.Comment, err error) {
	removed, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assigneeID)
	if err != nil {
		return false, nil, err
	}

	assignee, err := user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return false, nil, err
	}
	issue.AssigneeID = assigneeID
	issue.Assignee = assignee

	notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, removed, comment)

	return removed, comment, err
}

// UpdateAssignees is a helper function to add or delete one or multiple issue assignee(s)
// Deleting is done the GitHub way (quote from their api documentation):
// https://developer.github.com/v3/issues/#edit-an-issue
// "assignees" (array): Logins for Users to assign to this issue.
// Pass one or more user logins to replace the set of assignees on this Issue.
// Send an empty array ([]) to clear all assignees from the Issue.
func UpdateAssignees(ctx context.Context, issue *issues_model.Issue, oneAssignee string, multipleAssignees []string, doer *user_model.User) (err error) {
	uniqueAssignees := container.SetOf(multipleAssignees...)

	// Keep the old assignee thingy for compatibility reasons
	if oneAssignee != "" {
		uniqueAssignees.Add(oneAssignee)
	}

	// Loop through all assignees to add them
	allNewAssignees := make([]*user_model.User, 0, len(uniqueAssignees))
	for _, assigneeName := range uniqueAssignees.Values() {
		assignee, err := user_model.GetUserByName(ctx, assigneeName)
		if err != nil {
			return err
		}

		if user_model.IsUserBlockedBy(ctx, doer, assignee.ID) {
			return user_model.ErrBlockedUser
		}

		allNewAssignees = append(allNewAssignees, assignee)
	}

	// Delete all old assignees not passed
	if err = DeleteNotPassedAssignee(ctx, issue, doer, allNewAssignees); err != nil {
		return err
	}

	// Add all new assignees
	// Update the assignee. The function will check if the user exists, is already
	// assigned (which he shouldn't as we deleted all assignees before) and
	// has access to the repo.
	for _, assignee := range allNewAssignees {
		// Extra method to prevent double adding (which would result in removing)
		_, err = AddAssigneeIfNotAssigned(ctx, issue, doer, assignee.ID, true)
		if err != nil {
			return err
		}
	}

	return err
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't already assigned to the issue.
// Also checks for access of assigned user
func AddAssigneeIfNotAssigned(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64, notify bool) (comment *issues_model.Comment, err error) {
	assignee, err := user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return nil, err
	}

	// Check if the user is already assigned
	isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assignee)
	if err != nil {
		return nil, err
	}
	if isAssigned {
		// nothing to do
		return nil, nil //nolint:nilnil // return nil because the user is already assigned
	}

	valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo, issue.IsPull)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name}
	}

	if notify {
		_, comment, err = ToggleAssigneeWithNotify(ctx, issue, doer, assigneeID)
		return comment, err
	}
	_, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assigneeID)
	return comment, err
}
