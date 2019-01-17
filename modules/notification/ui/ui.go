// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ui

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
)

type (
	notificationService struct {
		base.NullNotifier
		issueQueue chan issueNotificationOpts
	}

	issueNotificationOpts struct {
		issue                *models.Issue
		notificationAuthorID int64
	}
)

var (
	_ base.Notifier = &notificationService{}
)

// NewNotifier create a new notificationService notifier
func NewNotifier() base.Notifier {
	return &notificationService{
		issueQueue: make(chan issueNotificationOpts, 100),
	}
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

func (ns *notificationService) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	ns.issueQueue <- issueNotificationOpts{
		issue,
		doer.ID,
	}
}

func (ns *notificationService) NotifyNewIssue(issue *models.Issue) {
	ns.issueQueue <- issueNotificationOpts{
		issue,
		issue.Poster.ID,
	}
}

func (ns *notificationService) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, isClosed bool) {
	ns.issueQueue <- issueNotificationOpts{
		issue,
		doer.ID,
	}
}

func (ns *notificationService) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, gitRepo *git.Repository) {
	ns.issueQueue <- issueNotificationOpts{
		pr.Issue,
		doer.ID,
	}
}

func (ns *notificationService) NotifyNewPullRequest(pr *models.PullRequest) {
	ns.issueQueue <- issueNotificationOpts{
		pr.Issue,
		pr.Issue.PosterID,
	}
}

func (ns *notificationService) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, c *models.Comment) {
	ns.issueQueue <- issueNotificationOpts{
		pr.Issue,
		r.Reviewer.ID,
	}
}
