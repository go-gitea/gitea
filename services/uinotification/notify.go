// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uinotification

import (
	"context"
	"slices"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

type (
	notificationService struct {
		notify_service.NullNotifier
		queue *queue.WorkerPoolQueue[notificationOpts]
	}

	notificationOpts struct {
		Source               activities_model.NotificationSource
		IssueID              int64
		CommentID            int64
		CommitID             string // commit ID for commit notifications
		RepoID               int64
		ReleaseID            int64
		NotificationAuthorID int64
		ReceiverID           int64 // 0 -- ALL Watcher
	}
)

func Init() error {
	notify_service.RegisterNotifier(NewNotifier())

	return nil
}

var _ notify_service.Notifier = &notificationService{}

// NewNotifier create a new notificationService notifier
func NewNotifier() notify_service.Notifier {
	ns := &notificationService{}
	ns.queue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "notification-service", handler)
	if ns.queue == nil {
		log.Fatal("Unable to create notification-service queue")
	}
	return ns
}

func handler(items ...notificationOpts) []notificationOpts {
	for _, opts := range items {
		switch opts.Source {
		case activities_model.NotificationSourceRepository:
			if err := activities_model.CreateRepoTransferNotification(db.DefaultContext, opts.NotificationAuthorID, opts.RepoID, opts.ReceiverID); err != nil {
				log.Error("CreateRepoTransferNotification: %v", err)
			}
		case activities_model.NotificationSourceCommit:
			if err := activities_model.CreateCommitNotifications(db.DefaultContext, opts.NotificationAuthorID, opts.RepoID, opts.CommitID, opts.ReceiverID); err != nil {
				log.Error("Was unable to create commit notification: %v", err)
			}
		case activities_model.NotificationSourceRelease:
			if err := activities_model.CreateOrUpdateReleaseNotifications(db.DefaultContext, opts.NotificationAuthorID, opts.RepoID, opts.ReleaseID, opts.ReceiverID); err != nil {
				log.Error("Was unable to create release notification: %v", err)
			}
		case activities_model.NotificationSourceIssue, activities_model.NotificationSourcePullRequest:
			fallthrough
		default:
			if err := activities_model.CreateOrUpdateIssueNotifications(db.DefaultContext, opts.IssueID, opts.CommentID, opts.NotificationAuthorID, opts.ReceiverID); err != nil {
				log.Error("Was unable to create issue notification: %v", err)
			}
		}
	}
	return nil
}

func (ns *notificationService) Run() {
	go graceful.GetManager().RunWithCancel(ns.queue) // TODO: using "go" here doesn't seem right, just leave it as old code
}

func (ns *notificationService) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	opts := notificationOpts{
		Source:               util.Iif(issue.IsPull, activities_model.NotificationSourcePullRequest, activities_model.NotificationSourceIssue),
		IssueID:              issue.ID,
		RepoID:               issue.RepoID,
		NotificationAuthorID: doer.ID,
	}
	if comment != nil {
		opts.CommentID = comment.ID
	}
	_ = ns.queue.Push(opts)
	for _, mention := range mentions {
		opts.ReceiverID = mention.ID
		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourceIssue,
		RepoID:               issue.RepoID,
		IssueID:              issue.ID,
		NotificationAuthorID: issue.Poster.ID,
	}
	_ = ns.queue.Push(opts)
	for _, mention := range mentions {
		opts.ReceiverID = mention.ID
		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	_ = ns.queue.Push(notificationOpts{
		Source:               util.Iif(issue.IsPull, activities_model.NotificationSourcePullRequest, activities_model.NotificationSourceIssue),
		IssueID:              issue.ID,
		NotificationAuthorID: doer.ID,
		CommentID:            actionComment.ID,
	})
}

func (ns *notificationService) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("issue.LoadPullRequest: %v", err)
		return
	}
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issue.PullRequest.IsWorkInProgress(ctx) {
		_ = ns.queue.Push(notificationOpts{
			Source:               util.Iif(issue.IsPull, activities_model.NotificationSourcePullRequest, activities_model.NotificationSourceIssue),
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
		})
	}
}

func (ns *notificationService) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	_ = ns.queue.Push(notificationOpts{
		Source:               activities_model.NotificationSourcePullRequest,
		IssueID:              pr.Issue.ID,
		NotificationAuthorID: doer.ID,
	})
}

func (ns *notificationService) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	ns.MergePullRequest(ctx, doer, pr)
}

func (ns *notificationService) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
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
		_ = ns.queue.Push(notificationOpts{
			Source:               activities_model.NotificationSourcePullRequest,
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: pr.Issue.PosterID,
			ReceiverID:           receiverID,
		})
	}
}

func (ns *notificationService) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, c *issues_model.Comment, mentions []*user_model.User) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourcePullRequest,
		IssueID:              pr.Issue.ID,
		NotificationAuthorID: r.Reviewer.ID,
	}
	if c != nil {
		opts.CommentID = c.ID
	}
	_ = ns.queue.Push(opts)
	for _, mention := range mentions {
		opts.ReceiverID = mention.ID
		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, c *issues_model.Comment, mentions []*user_model.User) {
	for _, mention := range mentions {
		_ = ns.queue.Push(notificationOpts{
			Source:               activities_model.NotificationSourcePullRequest,
			IssueID:              pr.Issue.ID,
			NotificationAuthorID: c.Poster.ID,
			CommentID:            c.ID,
			ReceiverID:           mention.ID,
		})
	}
}

func (ns *notificationService) PullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourcePullRequest,
		IssueID:              pr.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.queue.Push(opts)
}

func (ns *notificationService) PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourcePullRequest,
		IssueID:              review.IssueID,
		NotificationAuthorID: doer.ID,
		CommentID:            comment.ID,
	}
	_ = ns.queue.Push(opts)
}

func (ns *notificationService) IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	if !removed && doer.ID != assignee.ID {
		opts := notificationOpts{
			Source:               activities_model.NotificationSourceIssue,
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
			ReceiverID:           assignee.ID,
		}

		if comment != nil {
			opts.CommentID = comment.ID
		}

		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	if isRequest {
		opts := notificationOpts{
			Source:               activities_model.NotificationSourcePullRequest,
			IssueID:              issue.ID,
			NotificationAuthorID: doer.ID,
			ReceiverID:           reviewer.ID,
		}

		if comment != nil {
			opts.CommentID = comment.ID
		}

		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourceRepository,
		RepoID:               repo.ID,
		NotificationAuthorID: doer.ID,
	}

	if newOwner.IsOrganization() {
		users, err := organization.GetUsersWhoCanCreateOrgRepo(ctx, newOwner.ID)
		if err != nil {
			log.Error("GetUsersWhoCanCreateOrgRepo: %v", err)
			return
		}
		for i := range users {
			opts.ReceiverID = users[i].ID
			_ = ns.queue.Push(opts)
		}
	} else {
		opts.ReceiverID = newOwner.ID
		_ = ns.queue.Push(opts)
	}
}

func (ns *notificationService) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if len(commits.Commits) == 0 {
		return
	}

	for _, commit := range commits.Commits {
		mentions := references.FindAllMentionsMarkdown(commit.Message)
		receivers, err := user_model.GetUsersByUsernames(ctx, mentions)
		if err != nil {
			log.Error("GetUserIDsByNames: %v", err)
			return
		}

		notBlocked := make([]*user_model.User, 0, len(mentions))
		for _, user := range receivers {
			if !user_model.IsUserBlockedBy(ctx, repo.Owner, user.ID) {
				notBlocked = append(notBlocked, user)
			}
		}
		receivers = notBlocked

		for _, receiver := range receivers {
			perm, err := access_model.GetUserRepoPermission(ctx, repo, receiver)
			if err != nil {
				log.Error("GetUserRepoPermission [%d]: %w", receiver.ID, err)
				return
			}
			if !perm.CanRead(unit.TypeCode) {
				continue
			}

			opts := notificationOpts{
				Source:               activities_model.NotificationSourceCommit,
				RepoID:               repo.ID,
				CommitID:             commit.Sha1,
				NotificationAuthorID: pusher.ID,
				ReceiverID:           receiver.ID,
			}
			if err := ns.queue.Push(opts); err != nil {
				log.Error("PushCommits: %v", err)
			}
		}
	}
}

func (ns *notificationService) NewRelease(ctx context.Context, rel *repo_model.Release) {
	_ = rel.LoadPublisher(ctx)
	ns.UpdateRelease(ctx, rel.Publisher, rel)
}

func (ns *notificationService) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourceRelease,
		RepoID:               rel.RepoID,
		ReleaseID:            rel.ID,
		NotificationAuthorID: rel.PublisherID,
	}

	repoWatcherIDs, err := repo_model.GetRepoWatchersIDs(ctx, rel.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs: %v", err)
		return
	}

	repo, err := repo_model.GetRepositoryByID(ctx, rel.RepoID)
	if err != nil {
		log.Error("GetRepositoryByID: %v", err)
		return
	}
	if err := repo.LoadOwner(ctx); err != nil {
		log.Error("LoadOwner: %v", err)
		return
	}
	if !repo.Owner.IsOrganization() && !slices.Contains(repoWatcherIDs, repo.Owner.ID) && repo.Owner.ID != doer.ID {
		repoWatcherIDs = append(repoWatcherIDs, repo.Owner.ID)
	}

	for _, watcherID := range repoWatcherIDs {
		if watcherID == doer.ID {
			// Do not notify the publisher of the release
			continue
		}

		opts.ReceiverID = watcherID
		_ = ns.queue.Push(opts)
	}
}
