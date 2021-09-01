// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package comments

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/timeutil"
)

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *models.User, repo *models.Repository, issue *models.Issue, content string, attachments []string) (*models.Comment, error) {
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
	mentions, err := issue.FindAndUpdateIssueMentions(models.DefaultDBContext(), doer, comment.Content)
	if err != nil {
		return nil, err
	}

	models.SaveIssueContentHistory(doer.ID, issue.ID, comment.ID, timeutil.TimeStampNow(), comment.Content, true)

	notification.NotifyCreateIssueComment(doer, repo, issue, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(c *models.Comment, doer *models.User, oldContent string) error {
	if err := models.UpdateComment(c, doer); err != nil {
		return err
	}

	if c.Type == models.CommentTypeComment && c.Content != oldContent {
		models.SaveIssueContentHistory(doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
	}

	notification.NotifyUpdateComment(doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(doer *models.User, comment *models.Comment) error {
	if err := models.DeleteComment(comment); err != nil {
		return err
	}

	notification.NotifyDeleteComment(doer, comment)

	return nil
}
