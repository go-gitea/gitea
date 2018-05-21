// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mail

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

type mailReceiver struct {
}

var (
	receiver notification.NotifyReceiver = &mailReceiver{}
)

func init() {
	notification.RegisterReceiver(receiver)
}

func (m *mailReceiver) Run() {
}

func (m *mailReceiver) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	panic("not implementation")
}

func (m *mailReceiver) NotifyNewIssue(issue *models.Issue) {
	if err := issue.MailParticipants(); err != nil {
		log.Error(4, "MailParticipants: %v", err)
	}
}

func (m *mailReceiver) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	if err := issue.MailParticipants(); err != nil {
		log.Error(4, "MailParticipants: %v", err)
	}
}

func (m *mailReceiver) NotifyNewPullRequest(pr *models.PullRequest) {
	if err := pr.Issue.MailParticipants(); err != nil {
		log.Error(4, "MailParticipants: %v", err)
	}
}

func (m *mailReceiver) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseRepo *git.Repository) {
}
