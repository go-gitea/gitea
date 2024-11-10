// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CloseIssue close and issue.
func CloseIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ChangeIssueStatus(ctx, issue, doer, true)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) {
			if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
				log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
			}
		}
		return err
	}

	if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, true)

	return nil
}

// ReopenIssue reopen an issue.
// FIXME: If some issues dependent this one are closed, should we also reopen them?
func ReopenIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ChangeIssueStatus(ctx, issue, doer, false)
	if err != nil {
		return err
	}

	if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, false)

	return nil
}
