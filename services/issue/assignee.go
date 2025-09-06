// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, assignees []*user_model.User) (err error) {
	var found bool
	oriAssignes := make([]*user_model.User, len(issue.Assignees))
	_ = copy(oriAssignes, issue.Assignees)

	for _, assignee := range oriAssignes {
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
