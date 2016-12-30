// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notification

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
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
	// Service is the notification service
	Service = &notificationService{
		issueQueue: make(chan issueNotificationOpts, 100),
	}
)

func init() {
	go Service.Run()
}

func (ns *notificationService) Run() {
	for {
		select {
		case opts := <-ns.issueQueue:
			if err := models.CreateOrUpdateIssueNotifications(opts.issue, opts.notificationAuthorID); err != nil {
				log.Error(4, "Was unable to create issue notification: %v", err)
			}
		}
	}
}

func (ns *notificationService) NotifyIssue(issue *models.Issue, notificationAuthorID int64) {
	ns.issueQueue <- issueNotificationOpts{
		issue,
		notificationAuthorID,
	}
}
