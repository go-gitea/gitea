// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
)

// MailParticipantsComment sends new comment emails to repository watchers
// and mentioned people.
func MailParticipantsComment(c *models.Comment, opType models.ActionType, issue *models.Issue) error {
	return mailParticipantsComment(models.DefaultDBContext(), c, opType, issue)
}

func mailParticipantsComment(ctx models.DBContext, c *models.Comment, opType models.ActionType, issue *models.Issue) (err error) {
	mentions := markup.FindAllMentions(c.Content)
	if err = models.UpdateIssueMentions(ctx, c.IssueID, mentions); err != nil {
		return fmt.Errorf("UpdateIssueMentions [%d]: %v", c.IssueID, err)
	}

	if len(c.Content) > 0 {
		if err = mailIssueCommentToParticipants(issue, c.Poster, c.Content, c, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	switch opType {
	case models.ActionCloseIssue:
		ct := fmt.Sprintf("Closed #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(issue, c.Poster, ct, c, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	case models.ActionReopenIssue:
		ct := fmt.Sprintf("Reopened #%d.", issue.Index)
		if err = mailIssueCommentToParticipants(issue, c.Poster, ct, c, mentions); err != nil {
			log.Error("mailIssueCommentToParticipants: %v", err)
		}
	}

	return nil
}
