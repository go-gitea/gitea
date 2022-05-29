// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package comments

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/timeutil"
)

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *user_model.User, repo *repo_model.Repository, issue *models.Issue, content string, attachments []string) (*models.Comment, error) {
	comment, err := models.CreateComment(&models.CreateCommentOptions{
		Type:        models.CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, err
	}

	mentions, err := models.FindAndUpdateIssueMentions(db.DefaultContext, issue, doer, comment.Content)
	if err != nil {
		return nil, err
	}

	notification.NotifyCreateIssueComment(doer, repo, issue, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(c *models.Comment, doer *user_model.User, oldContent string) error {
	needsContentHistory := c.Content != oldContent &&
		(c.Type == models.CommentTypeComment || c.Type == models.CommentTypeReview || c.Type == models.CommentTypeCode)
	if needsContentHistory {
		hasContentHistory, err := issues.HasIssueContentHistory(db.DefaultContext, c.IssueID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = issues.SaveIssueContentHistory(db.DefaultContext, c.PosterID, c.IssueID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := models.UpdateComment(c, doer); err != nil {
		return err
	}

	if needsContentHistory {
		err := issues.SaveIssueContentHistory(db.DefaultContext, doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
		if err != nil {
			return err
		}
	}

	notification.NotifyUpdateComment(doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(doer *user_model.User, comment *models.Comment) error {
	if err := models.DeleteComment(comment); err != nil {
		return err
	}

	notification.NotifyDeleteComment(doer, comment)

	return nil
}
