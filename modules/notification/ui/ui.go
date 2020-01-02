// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ui

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
)

type (
	notificationService struct {
		base.NullNotifier
		issueQueue chan issueNotificationOpts
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
	return &notificationService{
		issueQueue: make(chan issueNotificationOpts, 100),
	}
}

func (ns *notificationService) Run() {
	for opts := range ns.issueQueue {
		if err := models.CreateOrUpdateIssueNotifications(opts.issueID, opts.commentID, opts.notificationAuthorID); err != nil {
			log.Error("Was unable to create issue notification: %v", err)
		}
	}
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
	ns.issueQueue <- opts
}

func (ns *notificationService) NotifyNewIssue(issue *models.Issue) {
	ns.issueQueue <- issueNotificationOpts{
		issueID:              issue.ID,
		notificationAuthorID: issue.Poster.ID,
	}
}

func (ns *notificationService) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
	ns.issueQueue <- issueNotificationOpts{
		issueID:              issue.ID,
		notificationAuthorID: doer.ID,
	}
}

func (ns *notificationService) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, gitRepo *git.Repository) {
	ns.issueQueue <- issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: doer.ID,
	}
}

func (ns *notificationService) NotifyNewPullRequest(pr *models.PullRequest) {
	ns.issueQueue <- issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: pr.Issue.PosterID,
	}
}

func (ns *notificationService) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, c *models.Comment) {
	var opts = issueNotificationOpts{
		issueID:              pr.Issue.ID,
		notificationAuthorID: r.Reviewer.ID,
	}
	if c != nil {
		opts.commentID = c.ID
	}
	ns.issueQueue <- opts
}
