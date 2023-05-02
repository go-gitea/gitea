// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/timeutil"
)

// CreateComment creates comment of issue or commit.
func CreateComment(ctx context.Context, opts *issues_model.CreateCommentOptions) (comment *issues_model.Comment, err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	comment, err = issues_model.CreateComment(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err = committer.Commit(); err != nil {
		return nil, err
	}

	return comment, nil
}

// CreateRefComment creates a commit reference comment to issue.
func CreateRefComment(doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content, commitSHA string) error {
	if len(commitSHA) == 0 {
		return fmt.Errorf("cannot create reference with empty commit SHA")
	}

	// Check if same reference from same commit has already existed.
	has, err := db.GetEngine(db.DefaultContext).Get(&issues_model.Comment{
		Type:      issues_model.CommentTypeCommitRef,
		IssueID:   issue.ID,
		CommitSHA: commitSHA,
	})
	if err != nil {
		return fmt.Errorf("check reference comment: %w", err)
	} else if has {
		return nil
	}

	_, err = CreateComment(db.DefaultContext, &issues_model.CreateCommentOptions{
		Type:      issues_model.CommentTypeCommitRef,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: commitSHA,
		Content:   content,
	})
	return err
}

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content string, attachments []string) (*issues_model.Comment, error) {
	comment, err := CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:        issues_model.CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comment.Content)
	if err != nil {
		return nil, err
	}

	notification.NotifyCreateIssueComment(ctx, doer, repo, issue, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(ctx context.Context, c *issues_model.Comment, doer *user_model.User, oldContent string) error {
	needsContentHistory := c.Content != oldContent && c.Type.HasContentSupport()
	if needsContentHistory {
		hasContentHistory, err := issues_model.HasIssueContentHistory(ctx, c.IssueID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = issues_model.SaveIssueContentHistory(ctx, c.PosterID, c.IssueID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := issues_model.UpdateComment(c, doer); err != nil {
		return err
	}

	if needsContentHistory {
		err := issues_model.SaveIssueContentHistory(ctx, doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
		if err != nil {
			return err
		}
	}

	notification.NotifyUpdateComment(ctx, doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) error {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return issues_model.DeleteComment(ctx, comment)
	})
	if err != nil {
		return err
	}

	notification.NotifyDeleteComment(ctx, doer, comment)

	return nil
}
