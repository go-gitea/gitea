// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CloseIssue close an issue.
func CloseIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	var comment *issues_model.Comment
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		comment, err = issues_model.CloseIssue(ctx, issue, doer)
		if err != nil {
			if issues_model.IsErrDependenciesLeft(err) {
				if _, err := issues_model.FinishIssueStopwatch(ctx, doer, issue); err != nil {
					log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
				}
			}
			return err
		}

		_, err = issues_model.FinishIssueStopwatch(ctx, doer, issue)
		return err
	}); err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, true)

	return nil
}

// ReopenIssue reopen an issue.
// FIXME: If some issues dependent this one are closed, should we also reopen them?
func ReopenIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ReopenIssue(ctx, issue, doer)
	if err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, false)

	return nil
}

// MoveIssueToFirstReview moves an issue or pull request into the first review workflow state.
func MoveIssueToFirstReview(ctx context.Context, issue *issues_model.Issue) error {
	if err := issues_model.SetIssueAsFirstReview(ctx, issue); err != nil {
		return err
	}
	issue_indexer.UpdateIssueIndexer(ctx, issue.ID)
	return nil
}

// MoveIssueToSecondReview moves an issue or pull request into the second review workflow state.
func MoveIssueToSecondReview(ctx context.Context, issue *issues_model.Issue) error {
	if err := issues_model.SetIssueAsSecondReview(ctx, issue); err != nil {
		return err
	}
	issue_indexer.UpdateIssueIndexer(ctx, issue.ID)
	return nil
}

// ReopenReviewIssue moves a first- or second-review issue back to open.
func ReopenReviewIssue(ctx context.Context, issue *issues_model.Issue) error {
	if err := issues_model.ReopenReviewIssue(ctx, issue); err != nil {
		return err
	}
	issue_indexer.UpdateIssueIndexer(ctx, issue.ID)
	return nil
}
