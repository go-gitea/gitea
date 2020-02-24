// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
)

// MailParticipantsComment sends new comment emails to repository watchers
// and mentioned people.
func MailParticipantsComment(c *models.Comment, opType models.ActionType, issue *models.Issue) error {
	return mailParticipantsComment(models.DefaultDBContext(), c, opType, issue)
}

func mailParticipantsComment(ctx models.DBContext, c *models.Comment, opType models.ActionType, issue *models.Issue) (err error) {
	rawMentions := references.FindAllMentionsMarkdown(c.Content)
	userMentions, err := issue.ResolveMentionsByVisibility(ctx, c.Poster, rawMentions)
	if err != nil {
		return fmt.Errorf("ResolveMentionsByVisibility [%d]: %v", c.IssueID, err)
	}
	if err = models.UpdateIssueMentions(ctx, c.IssueID, userMentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", c.IssueID, err)
	}
	mentions := make([]int64, len(userMentions))
	for i, u := range userMentions {
		mentions[i] = u.ID
	}
	if err = mailIssueCommentToParticipants(
		&mailCommentContext{
			Issue:      issue,
			Doer:       c.Poster,
			ActionType: opType,
			Content:    c.Content,
			Comment:    c,
		}, mentions); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}
