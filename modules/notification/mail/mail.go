// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mail

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/services/mailer"
)

type mailNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &mailNotifier{}
)

// NewNotifier create a new mailNotifier notifier
func NewNotifier() base.Notifier {
	return &mailNotifier{}
}

func (m *mailNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentIssue
	} else if comment.Type == models.CommentTypeCode {
		act = models.ActionCommentIssue
	}

	if err := mailer.MailParticipantsComment(comment, act, issue); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyNewIssue(issue *models.Issue) {
	if err := mailer.MailParticipants(issue, issue.Poster, models.ActionCreateIssue); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, isClosed bool) {
	var actionType models.ActionType
	if issue.IsPull {
		if isClosed {
			actionType = models.ActionClosePullRequest
		} else {
			actionType = models.ActionReopenPullRequest
		}
	} else {
		if isClosed {
			actionType = models.ActionCloseIssue
		} else {
			actionType = models.ActionReopenIssue
		}
	}

	if err := mailer.MailParticipants(issue, doer, actionType); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
	if err := mailer.MailParticipants(pr.Issue, pr.Issue.Poster, models.ActionCreatePullRequest); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, comment *models.Comment) {
	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentIssue
	}
	if err := mailer.MailParticipantsComment(comment, act, pr.Issue); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}
