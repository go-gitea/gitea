// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CloseIssue closes an issue
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

// CloseIssueWithComment close an issue with comment
func CloseIssueWithComment(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID, commentContent string, attachments []string) (*issues_model.Comment, error) {
	var refComment, createdComment *issues_model.Comment
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if commentContent != "" || len(attachments) > 0 {
			createdComment, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Type:        issues_model.CommentTypeComment,
				Doer:        doer,
				Repo:        issue.Repo,
				Issue:       issue,
				Content:     commentContent,
				Attachments: attachments,
			})
			if err != nil {
				return err
			}
		}

		refComment, err = issues_model.CloseIssue(ctx, issue, doer)
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
		return nil, err
	}

	if createdComment != nil {
		if err := notifyCommentCreated(ctx, doer, issue.Repo, issue, createdComment); err != nil {
			log.Error("Unable to notify comment created for issue[%d]#%d: %v", issue.ID, issue.Index, err)
		}
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, refComment, true)

	return createdComment, nil
}

// ReopenIssue reopen an issue
// FIXME: If some issues dependent this one are closed, should we also reopen them?
func ReopenIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ReopenIssue(ctx, issue, doer)
	if err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, false)

	return nil
}

// ReopenIssueWithComment reopen an issue with a comment.
// FIXME: If some issues dependent this one are closed, should we also reopen them?
func ReopenIssueWithComment(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID, commentContent string, attachments []string) (*issues_model.Comment, error) {
	var createdComment *issues_model.Comment
	refComment, err := db.WithTx2(ctx, func(ctx context.Context) (*issues_model.Comment, error) {
		var err error
		if commentContent != "" || len(attachments) > 0 {
			createdComment, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Type:        issues_model.CommentTypeComment,
				Doer:        doer,
				Repo:        issue.Repo,
				Issue:       issue,
				Content:     commentContent,
				Attachments: attachments,
			})
			if err != nil {
				return nil, err
			}
		}

		return issues_model.ReopenIssue(ctx, issue, doer)
	})
	if err != nil {
		return nil, err
	}

	if createdComment != nil {
		if err := notifyCommentCreated(ctx, doer, issue.Repo, issue, createdComment); err != nil {
			log.Error("Unable to notify comment created for issue[%d]#%d: %v", issue.ID, issue.Index, err)
		}
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, refComment, false)

	return createdComment, nil
}
