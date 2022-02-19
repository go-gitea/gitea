// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ui

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
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
		IssueID              int64
		CommentID            int64
		NotificationAuthorID int64
		ReceiverID           int64 // 0 -- ALL Watcher
	}
)

var _ base.Notifier = &notificationService{}

// NewNotifier create a new notificationService notifier
func NewNotifier() base.Notifier {
	ns := &notificationService{}
	ns.issueQueue = queue.CreateQueue("notification-service", ns.handle, issueNotificationOpts{})
	return ns
}

func (ns *notificationService) handle(data ...queue.Data) []queue.Data {
	for _, datum := range data {
		opts := datum.(issueNotificationOpts)
		if err := models.CreateOrUpdateIssueNotifications(opts.IssueID, opts.CommentID, opts.NotificationAuthorID, opts.ReceiverID); err != nil {
			log.Error("Was unable to create issue notification: %v", err)
		}
	}
	return nil
}

func (ns *notificationService) Run() {
	graceful.GetManager().RunWithShutdownFns(ns.issueQueue.Run)
}

func (ns *notificationService) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*user_model.User) {
	opts := issueNotificationOpts{
		IssueID:              issue.ID,
		NotificationAuthorID: doer.ID,
	}
	if comment != nil {
		opts.CommentID = comment.ID
	}
	_ = ns.issueQueue.Push(opts)
	for _, mention := range mentions {
		opts := issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
			ReceiverID:           mention.ID,
		}
		if comment != nil {
			opts.CommentID = comment.ID
		}
		_ = ns.issueQueue.Push(opts)
	}
}

func (ns *notificationService) NotifyNewIssue(issue *models.Issue, mentions []*user_model.User) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		IssueID:              issue.ID,
		NotificationAuthorID: issue.Poster.ID,
	})
	for _, mention := range mentions {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: issue.Poster.ID,
			ReceiverID:           mention.ID,
		})
	}
}

func (ns *notificationService) NotifyIssueChangeStatus(doer *user_model.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		IssueID:              issue.ID,
		NotificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) NotifyIssueChangeTitle(doer *user_model.User, issue *models.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && models.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress() {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
		})
	}
}

func (ns *notificationService) NotifyMergePullRequest(pr *models.PullRequest, doer *user_model.User) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		IssueID:              pr.Issue.ID,
		NotificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) NotifyNewPullRequest(pr *models.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("Unable to load issue: %d for pr: %d: Error: %v", pr.IssueID, pr.ID, err)
		return
	}
	toNotify := make(map[int64]struct{}, 32)
	repoWatchers, err := repo_model.GetRepoWatchersIDs(db.DefaultContext, pr.Issue.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs: %v", err)
		return
	}
	for _, id := range repoWatchers {
		toNotify[id] = struct{}{}
	}
	issueParticipants, err := models.GetParticipantsIDsByIssueID(pr.IssueID)
	if err != nil {
		log.Error("GetParticipantsIDsByIssueID: %v", err)
		return
	}
	for _, id := range issueParticipants {
		toNotify[id] = struct{}{}
	}
	delete(toNotify, pr.Issue.PosterID)
	for _, mention := range mentions {
		toNotify[mention.ID] = struct{}{}
	}
	for receiverID := range toNotify {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: pr.Issue.PosterID,
			ReceiverID:           receiverID,
		})
	}
}

func (ns *notificationService) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, c *models.Comment, mentions []*user_model.User) {
	opts := issueNotificationOpts{
		IssueID:              pr.Issue.ID,
		NotificationAuthorID: r.Reviewer.ID,
	}
	if c != nil {
		opts.CommentID = c.ID
	}
	_ = ns.issueQueue.Push(opts)
	for _, mention := range mentions {
		opts := issueNotificationOpts{
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: r.Reviewer.ID,
			ReceiverID:           mention.ID,
		}
		if c != nil {
			opts.CommentID = c.ID
		}
		_ = ns.issueQueue.Push(opts)
	}
}

func (ns *notificationService) NotifyPullRequestCodeComment(pr *models.PullRequest, c *models.Comment, mentions []*user_model.User) {
	for _, mention := range mentions {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: c.Poster.ID,
			CommentID:            c.ID,
			ReceiverID:           mention.ID,
		})
	}
}

func (ns *notificationService) NotifyPullRequestPushCommits(doer *user_model.User, pr *models.PullRequest, comment *models.Comment) {
	opts := issueNotificationOpts{
		IssueID:              pr.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.issueQueue.Push(opts)
}

func (ns *notificationService) NotifyPullRevieweDismiss(doer *user_model.User, review *models.Review, comment *models.Comment) {
	opts := issueNotificationOpts{
		IssueID:              review.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.issueQueue.Push(opts)
}

func (ns *notificationService) NotifyIssueChangeAssignee(doer *user_model.User, issue *models.Issue, assignee *user_model.User, removed bool, comment *models.Comment) {
	if !removed && doer.ID != assignee.ID {
		opts := issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
			ReceiverID:           assignee.ID,
		}

		if comment != nil {
			opts.CommentID = comment.ID
		}

		_ = ns.issueQueue.Push(opts)
	}
}

func (ns *notificationService) NotifyPullReviewRequest(doer *user_model.User, issue *models.Issue, reviewer *user_model.User, isRequest bool, comment *models.Comment) {
	if isRequest {
		opts := issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
			ReceiverID:           reviewer.ID,
		}

		if comment != nil {
			opts.CommentID = comment.ID
		}

		_ = ns.issueQueue.Push(opts)
	}
}

func (ns *notificationService) NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *repo_model.Repository) {
	if err := models.CreateRepoTransferNotification(doer, newOwner, repo); err != nil {
		log.Error("NotifyRepoPendingTransfer: %v", err)
	}
}
