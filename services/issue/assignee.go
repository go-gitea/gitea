// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	notify_service "gitea.dev/services/notify"
)

func toBeRemovedAssignees(issue *issues_model.Issue, assignees []*user_model.User) (toBeRemovedAssignees []*user_model.User) {
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
			toBeRemovedAssignees = append(toBeRemovedAssignees, assignee)
		}
	}
	return toBeRemovedAssignees
}

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assignees []*user_model.User) (err error) {
	toBeRemoved := toBeRemovedAssignees(issue, assignees)

	for _, assignee := range toBeRemoved {
		// This function also does comments and hooks, which is why we call it separately instead of directly removing the assignees here
		removed, comment, err := ToggleAssignee(ctx, issue, doer, assignee)
		if err != nil {
			return err
		}
		if removed {
			notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, true, comment)
		}
	}

	return nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssignee(ctx context.Context, issue *issues_model.Issue, doer, assignee *user_model.User) (removed bool, comment *issues_model.Comment, err error) {
	removed, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assignee.ID)
	if err != nil {
		return false, nil, err
	}

	issue.AssigneeID = assignee.ID
	issue.Assignee = assignee

	return removed, comment, nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssigneeWithNotify(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeID int64) (removed bool, comment *issues_model.Comment, err error) {
	assignee, err := user_model.GetUserByID(ctx, assigneeID)
	if err != nil {
		return false, nil, err
	}

	removed, comment, err = ToggleAssignee(ctx, issue, doer, assignee)
	if err != nil {
		return false, nil, err
	}

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

		if err := validateAssignee(ctx, issue, doer, assignee); err != nil {
			return err
		}

		allNewAssignees = append(allNewAssignees, assignee)
	}

	assigneeCommentMap := make(map[int64]*issues_model.Comment)
	assigneeRemovedCommentMap := make(map[int64]*issues_model.Comment)
	assigneeRemoved := make(map[int64]*user_model.User)
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// Delete all old assignees not passed.
		toBeRemoved := toBeRemovedAssignees(issue, allNewAssignees)

		for _, assignee := range toBeRemoved {
			// This function also does comments and hooks, which is why we call it separately instead of directly removing the assignees here
			removed, comment, err := ToggleAssignee(ctx, issue, doer, assignee)
			if err != nil {
				return err
			}
			if removed {
				assigneeRemoved[assignee.ID] = assignee
				assigneeRemovedCommentMap[assignee.ID] = comment
			}
		}

		// Add all new assignees.
		// Update the assignee. The function will check if the user exists, is already
		// assigned (which he shouldn't as we deleted all assignees before) and
		// has access to the repo.
		for _, assignee := range allNewAssignees {
			// Extra method to prevent double adding (which would result in removing).
			comment, err := AddAssigneeIfNotAssigned(ctx, issue, doer, assignee)
			if err != nil {
				return err
			}
			assigneeCommentMap[assignee.ID] = comment
		}

		return nil
	}); err != nil {
		return err
	}

	for _, assignee := range assigneeRemoved {
		notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, true, assigneeRemovedCommentMap[assignee.ID])
	}

	for _, assignee := range allNewAssignees {
		comment := assigneeCommentMap[assignee.ID]
		if comment != nil {
			notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, false, comment)
		}
	}

	return nil
}

func validateAssignee(ctx context.Context, issue *issues_model.Issue, doer, assignee *user_model.User) error {
	if user_model.IsUserBlockedBy(ctx, doer, assignee.ID) {
		return user_model.ErrBlockedUser
	}

	valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo)
	if err != nil {
		return err
	}
	if !valid {
		return repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assignee.ID, RepoName: issue.Repo.Name}
	}

	return nil
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't already assigned to the issue.
// Also checks for access of assigned user
func AddAssigneeIfNotAssigned(ctx context.Context, issue *issues_model.Issue, doer, assignee *user_model.User) (comment *issues_model.Comment, err error) {
	// Check if the user is already assigned
	isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assignee.ID)
	if err != nil {
		return nil, err
	}
	if isAssigned {
		// nothing to do
		return nil, nil //nolint:nilnil // return nil because the user is already assigned
	}

	if err := validateAssignee(ctx, issue, doer, assignee); err != nil {
		return nil, err
	}

	_, comment, err = issues_model.ToggleIssueAssignee(ctx, issue, doer, assignee.ID)
	return comment, err
}

// AddAssignees adds multiple assignees to an issue atomically.
func AddAssignees(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeIDs []int64) error {
	assigneeCommentMap := make(map[int64]*issues_model.Comment)
	assignees := make(map[int64]*user_model.User)
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		for _, assigneeID := range assigneeIDs {
			isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assigneeID)
			if err != nil {
				return err
			}
			if isAssigned {
				continue
			}

			assignee, err := user_model.GetUserByID(ctx, assigneeID)
			if err != nil {
				return err
			}
			if err := validateAssignee(ctx, issue, doer, assignee); err != nil {
				return err
			}

			comment, err := AddAssigneeIfNotAssigned(ctx, issue, doer, assignee)
			if err != nil {
				return err
			}
			assignees[assigneeID] = assignee
			assigneeCommentMap[assigneeID] = comment
		}

		return nil
	}); err != nil {
		return err
	}

	if len(assignees) > 0 {
		for assigneeID, assignee := range assignees {
			notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, false, assigneeCommentMap[assigneeID])
		}
	}
	return nil
}

// RemoveAssignees removes multiple assignees from an issue atomically.
func RemoveAssignees(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assigneeIDs []int64) error {
	assigneeCommentMap := make(map[int64]*issues_model.Comment)
	assignees := make(map[int64]*user_model.User)
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		for _, assigneeID := range assigneeIDs {
			isAssigned, err := issues_model.IsUserAssignedToIssue(ctx, issue, assigneeID)
			if err != nil {
				return err
			}
			if !isAssigned {
				continue
			}
			removed, comment, err := issues_model.ToggleIssueAssignee(ctx, issue, doer, assigneeID)
			if err != nil {
				return err
			}
			if removed {
				assignee, err := user_model.GetUserByID(ctx, assigneeID)
				if err != nil {
					return err
				}
				assignees[assigneeID] = assignee
				assigneeCommentMap[assigneeID] = comment
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if len(assignees) > 0 {
		for assigneeID, assignee := range assignees {
			notify_service.IssueChangeAssignee(ctx, doer, issue, assignee, true, assigneeCommentMap[assigneeID])
		}
	}
	return nil
}
