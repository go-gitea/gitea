// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ui

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/queue"
)

type (
	notificationService struct {
		base.NullNotifier
		issueQueue queue.Queue
	}

	issueNotificationOpts struct {
		issueID              int64
		commentID            int64
		notificationAuthorID int64
	}
)

var (
	_ base.Notifier = &notificationService{}
)

// NewNotifier create a new notificationService notifier
func NewNotifier() base.Notifier {
	ns := &notificationService{}
	ns.issueQueue = queue.CreateQueue("notification-service", ns.handle, issueNotificationOpts{})
	return ns
}

func (ns *notificationService) handle(data ...queue.Data) {
	for _, datum := range data {
		opts := datum.(issueNotificationOpts)
		if err := models.CreateOrUpdateIssueNotifications(opts.issueID, opts.commentID, opts.notificationAuthorID); err != nil {
			log.Error("Was unable to create issue notification: %v", err)
		}
	}
}

func (ns *notificationService) Run() {
	graceful.GetManager().RunWithShutdownFns(ns.issueQueue.Run)
}

func (ns *notificationService) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	var opts = issueNotificationOpts{
		issueID:              issue.ID,
		notificationAuthorID: doer.ID,
	}
	if comment != nil {
		opts.commentID = comment.ID
	}
	_ = ns.issueQueue.Push(opts)
}

func (ns *notificationService) NotifyNewIssue(issue *models.Issue) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		issueID:              issue.ID,
		notificationAuthorID: issue.Poster.ID,
	})
}

func (ns *notificationService) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		issueID:              issue.ID,
		notificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) NotifyNewPullRequest(pr *models.PullRequest) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("Unable to load issue: %d for pr: %d: Error: %v", pr.IssueID, pr.ID, err)
		return
	}
	_ = ns.issueQueue.Push(issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: pr.Issue.PosterID,
	})
}

func (ns *notificationService) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, c *models.Comment) {
	var opts = issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: r.Reviewer.ID,
	}
	if c != nil {
		opts.commentID = c.ID
	}
	_ = ns.issueQueue.Push(opts)
}
