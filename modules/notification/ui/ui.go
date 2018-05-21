// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ui

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

type (
	notificationService struct {
		issueQueue chan issueNotificationOpts
	}

	issueNotificationOpts struct {
		issue                *models.Issue
		notificationAuthorID int64
	}
)

var (
	// service is the notification service
	service = &notificationService{
		issueQueue: make(chan issueNotificationOpts, 100),
	}
	_ notification.NotifyReceiver = &notificationService{}
)

func init() {
	notification.RegisterReceiver(service)
}

func (ns *notificationService) Run() {
	for {
		select {
		case opts := <-service.issueQueue:
			if err := models.CreateOrUpdateIssueNotifications(opts.issue, opts.notificationAuthorID); err != nil {
				log.Error(4, "Was unable to create issue notification: %v", err)
			}
		}
	}
}

func (w *notificationService) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	service.issueQueue <- issueNotificationOpts{
		issue,
		doer.ID,
	}
}

func (w *notificationService) NotifyNewIssue(issue *models.Issue) {
	service.issueQueue <- issueNotificationOpts{
		issue,
		issue.Poster.ID,
	}
}

func (w *notificationService) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	service.issueQueue <- issueNotificationOpts{
		issue,
		doer.ID,
	}
}

func (w *notificationService) NotifyMergePullRequest(*models.PullRequest, *models.User, *git.Repository) {
}

func (w *notificationService) NotifyNewPullRequest(pr *models.PullRequest) {
	service.issueQueue <- issueNotificationOpts{
		pr.Issue,
		pr.Issue.PosterID,
	}
}
