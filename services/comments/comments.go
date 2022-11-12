// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package comments

import (
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/timeutil"
)

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content string, attachments []string) (*issues_model.Comment, error) {
	comment, err := issues_model.CreateComment(&issues_model.CreateCommentOptions{
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

	mentions, err := issues_model.FindAndUpdateIssueMentions(db.DefaultContext, issue, doer, comment.Content)
	if err != nil {
		return nil, err
	}

	notification.NotifyCreateIssueComment(doer, repo, issue, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(c *issues_model.Comment, doer *user_model.User, oldContent string) error {
	needsContentHistory := c.Content != oldContent &&
		(c.Type == issues_model.CommentTypeComment || c.Type == issues_model.CommentTypeReview || c.Type == issues_model.CommentTypeCode)
	if needsContentHistory {
		hasContentHistory, err := issues_model.HasIssueContentHistory(db.DefaultContext, c.IssueID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = issues_model.SaveIssueContentHistory(db.DefaultContext, c.PosterID, c.IssueID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := issues_model.UpdateComment(c, doer); err != nil {
		return err
	}

	if needsContentHistory {
		err := issues_model.SaveIssueContentHistory(db.DefaultContext, doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
		if err != nil {
			return err
		}
	}

	notification.NotifyUpdateComment(doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(doer *user_model.User, comment *issues_model.Comment) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := issues_model.DeleteComment(ctx, comment); err != nil {
		return err
	}
	if err := committer.Commit(); err != nil {
		return err
	}

	notification.NotifyDeleteComment(doer, comment)

	return nil
}
