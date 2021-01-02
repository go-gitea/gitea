// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// MailParticipantsComment sends new comment emails to repository watchers
// and mentioned people.
func MailParticipantsComment(c *models.Comment, opType models.ActionType, issue *models.Issue, mentions []*models.User) error {
	return mailParticipantsComment(c, opType, issue, mentions)
}

func mailParticipantsComment(c *models.Comment, opType models.ActionType, issue *models.Issue, mentions []*models.User) (err error) {
	mentionedIDs := make([]int64, len(mentions))
	for i, u := range mentions {
		mentionedIDs[i] = u.ID
	}
	if err = mailIssueCommentToParticipants(
		&mailCommentContext{
			Issue:      issue,
			Doer:       c.Poster,
			ActionType: opType,
			Content:    c.Content,
			Comment:    c,
		}, mentionedIDs); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}
