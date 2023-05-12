// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ui

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/queue"
)

type (
	notificationService struct {
		base.NullNotifier
		issueQueue *queue.WorkerPoolQueue[issueNotificationOpts]
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
	ns.issueQueue = queue.CreateSimpleQueue("notification-service", handler)
	return ns
}

func handler(items ...issueNotificationOpts) []issueNotificationOpts {
	for _, opts := range items {
		if err := activities_model.CreateOrUpdateIssueNotifications(opts.IssueID, opts.CommentID, opts.NotificationAuthorID, opts.ReceiverID); err != nil {
			log.Error("Was unable to create issue notification: %v", err)
		}
	}
	return nil
}

func (ns *notificationService) Run() {
	go graceful.GetManager().RunWithShutdownFns(ns.issueQueue.Run)
}

func (ns *notificationService) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
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

func (ns *notificationService) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
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

func (ns *notificationService) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		IssueID:              issue.ID,
		NotificationAuthorID: doer.ID,
		CommentID:            actionComment.ID,
	})
}

func (ns *notificationService) NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress() {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
		})
	}
}

func (ns *notificationService) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	_ = ns.issueQueue.Push(issueNotificationOpts{
		IssueID:              pr.Issue.ID,
		NotificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ns.NotifyMergePullRequest(ctx, doer, pr)
}

func (ns *notificationService) NotifyNewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("Unable to load issue: %d for pr: %d: Error: %v", pr.IssueID, pr.ID, err)
		return
	}
	toNotify := make(container.Set[int64], 32)
	repoWatchers, err := repo_model.GetRepoWatchersIDs(ctx, pr.Issue.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs: %v", err)
		return
	}
	for _, id := range repoWatchers {
		toNotify.Add(id)
	}
	issueParticipants, err := issues_model.GetParticipantsIDsByIssueID(ctx, pr.IssueID)
	if err != nil {
		log.Error("GetParticipantsIDsByIssueID: %v", err)
		return
	}
	for _, id := range issueParticipants {
		toNotify.Add(id)
	}
	delete(toNotify, pr.Issue.PosterID)
	for _, mention := range mentions {
		toNotify.Add(mention.ID)
	}
	for receiverID := range toNotify {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: pr.Issue.PosterID,
			ReceiverID:           receiverID,
		})
	}
}

func (ns *notificationService) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, c *issues_model.Comment, mentions []*user_model.User) {
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

func (ns *notificationService) NotifyPullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, c *issues_model.Comment, mentions []*user_model.User) {
	for _, mention := range mentions {
		_ = ns.issueQueue.Push(issueNotificationOpts{
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: c.Poster.ID,
			CommentID:            c.ID,
			ReceiverID:           mention.ID,
		})
	}
}

func (ns *notificationService) NotifyPullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	opts := issueNotificationOpts{
		IssueID:              pr.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.issueQueue.Push(opts)
}

func (ns *notificationService) NotifyPullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	opts := issueNotificationOpts{
		IssueID:              review.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.issueQueue.Push(opts)
}

func (ns *notificationService) NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
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

func (ns *notificationService) NotifyPullReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
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

func (ns *notificationService) NotifyRepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return activities_model.CreateRepoTransferNotification(ctx, doer, newOwner, repo)
	})
	if err != nil {
		log.Error("CreateRepoTransferNotification: %v", err)
	}
}
