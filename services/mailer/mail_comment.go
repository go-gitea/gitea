// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"context"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// MailParticipantsComment sends new comment emails to repository watchers and mentioned people.
func MailParticipantsComment(ctx context.Context, c *models.Comment, opType models.ActionType, issue *models.Issue, mentions []*user_model.User) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	content := c.Content
	if c.Type == models.CommentTypePullRequestPush {
		content = ""
	}
	if err := mailIssueCommentToParticipants(
		&mailCommentContext{
			Context:    ctx,
			Issue:      issue,
			Doer:       c.Poster,
			ActionType: opType,
			Content:    content,
			Comment:    c,
		}, mentions); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}

// MailMentionsComment sends email to users mentioned in a code comment
func MailMentionsComment(ctx context.Context, pr *models.PullRequest, c *models.Comment, mentions []*user_model.User) (err error) {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	visited := make(map[int64]bool, len(mentions)+1)
	visited[c.Poster.ID] = true
	if err = mailIssueCommentBatch(
		&mailCommentContext{
			Context:    ctx,
			Issue:      pr.Issue,
			Doer:       c.Poster,
			ActionType: models.ActionCommentPull,
			Content:    c.Content,
			Comment:    c,
		}, mentions, visited, true); err != nil {
		log.Error("mailIssueCommentBatch: %v", err)
	}
	return nil
}
