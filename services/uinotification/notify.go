// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uinotification

import (
	"context"
	"slices"
	"strings"

	activities_model "gitea.dev/models/activities"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/references"
	"gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	notify_service "gitea.dev/services/notify"
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
		Title                string // snapshotted so the notification renders without loading the subject
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
	ctx := graceful.GetManager().ShutdownContext()
	for _, opts := range items {
		var notifiedIDs []int64
		var err error

		switch opts.Source {
		case activities_model.NotificationSourceRepository:
			notifiedIDs, err = notifyOne(opts.ReceiverID, func() (bool, error) {
				return activities_model.CreateRepoTransferNotification(ctx, opts.NotificationAuthorID, opts.RepoID, opts.ReceiverID, opts.Title)
			})
		case activities_model.NotificationSourceCommit:
			notifiedIDs, err = notifyOne(opts.ReceiverID, func() (bool, error) {
				return activities_model.CreateCommitNotification(ctx, opts.NotificationAuthorID, opts.RepoID, opts.CommitID, opts.ReceiverID, opts.Title)
			})
		case activities_model.NotificationSourceRelease:
			notifiedIDs, err = notifyOne(opts.ReceiverID, func() (bool, error) {
				return activities_model.CreateReleaseNotification(ctx, opts.NotificationAuthorID, opts.RepoID, opts.ReleaseID, opts.ReceiverID, opts.Title)
			})
		case activities_model.NotificationSourceIssue, activities_model.NotificationSourcePullRequest, 0:
			// Source==0 covers queue items persisted before this field existed, kept for rolling-upgrade safety.
			notifiedIDs, err = activities_model.CreateOrUpdateIssueNotifications(ctx, opts.IssueID, opts.CommentID, opts.NotificationAuthorID, opts.ReceiverID)
		default:
			setting.PanicInDevOrTesting("Unknown notification source: %v", opts.Source)
			continue
		}

		if err != nil {
			log.Error("Was unable to create notification (source %v): %v", opts.Source, err)
			continue
		}
		for _, userID := range notifiedIDs {
			notify_service.NotificationCountChange(ctx, userID)
		}
	}
	return nil
}

// notifyOne adapts the single-receiver creators to the notified-IDs shape the handler
// uses to push unread-count updates.
func notifyOne(receiverID int64, create func() (bool, error)) ([]int64, error) {
	changed, err := create()
	if err != nil || !changed {
		return nil, err
	}
	return []int64{receiverID}, nil
}

func (ns *notificationService) Run() {
	go graceful.GetManager().RunWithCancel(ns.queue) // TODO: using "go" here doesn't seem right, just leave it as old code
}

func (ns *notificationService) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	opts := notificationOpts{
		Source:               activities_model.NotificationSourceForIssue(issue),
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
		Source:               activities_model.NotificationSourceForIssue(issue),
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
			Source:               activities_model.NotificationSourceForIssue(issue),
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
			Source:               activities_model.NotificationSourceForIssue(issue),
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
		Title:                repo.FullName(),
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

func (ns *notificationService) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, _ *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if len(commits.Commits) == 0 {
		return
	}
	if err := repo.LoadOwner(ctx); err != nil {
		log.Error("LoadOwner [%d]: %v", repo.ID, err)
		return
	}

	// Collect unique mentions across all commits, and for each mention remember
	// which commit shas mentioned it. This lets us batch the user lookup, the
	// blocked-by check, and the permission check to one round-trip per user
	// instead of per (commit, user) pair.
	mentionToCommits := make(map[string][]string)
	for _, commit := range commits.Commits {
		for _, name := range references.FindAllMentionsMarkdown(commit.Message) {
			lower := strings.ToLower(name)
			mentionToCommits[lower] = append(mentionToCommits[lower], commit.Sha1)
		}
	}
	if len(mentionToCommits) == 0 {
		return
	}

	uniqueNames := make([]string, 0, len(mentionToCommits))
	for name := range mentionToCommits {
		uniqueNames = append(uniqueNames, name)
	}

	receivers, err := user_model.GetUsersByUsernames(ctx, uniqueNames)
	if err != nil {
		log.Error("GetUsersByUsernames: %v", err)
		return
	}

	receiverIDs := make([]int64, 0, len(receivers))
	receiverByID := make(map[int64]*user_model.User, len(receivers))
	for _, receiver := range receivers {
		if receiver.ID == pusher.ID { // no self-notification for mentioning yourself, as with every other source
			continue
		}
		receiverIDs = append(receiverIDs, receiver.ID)
		receiverByID[receiver.ID] = receiver
	}

	allowedIDs, err := filterRecipientsByRepoAccess(ctx, repo, receiverIDs, unit.TypeCode)
	if err != nil {
		log.Error("filterRecipientsByRepoAccess: %v", err)
		return
	}

	// The commit title is snapshotted here, while the message is still in hand, so the
	// notification never needs to open the git repository to render.
	commitTitles := make(map[string]string, len(commits.Commits))
	for _, commit := range commits.Commits {
		commitTitles[commit.Sha1], _ = git.SplitCommitTitleBody(commit.Message, 255)
	}

	for _, receiverID := range allowedIDs {
		for _, sha := range mentionToCommits[receiverByID[receiverID].LowerName] {
			opts := notificationOpts{
				Source:               activities_model.NotificationSourceCommit,
				RepoID:               repo.ID,
				CommitID:             sha,
				Title:                commitTitles[sha],
				NotificationAuthorID: pusher.ID,
				ReceiverID:           receiverID,
			}
			if err := ns.queue.Push(opts); err != nil {
				log.Error("PushCommits: %v", err)
			}
		}
	}
}

func (ns *notificationService) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadPublisher(ctx); err != nil {
		log.Error("NewRelease LoadPublisher: %v", err)
		return
	}
	ns.UpdateRelease(ctx, rel.Publisher, rel)
}

func (ns *notificationService) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	if rel.IsDraft {
		return
	}

	opts := notificationOpts{
		Source:               activities_model.NotificationSourceRelease,
		RepoID:               rel.RepoID,
		ReleaseID:            rel.ID,
		Title:                rel.Title,
		NotificationAuthorID: rel.PublisherID,
	}

	repoWatcherIDs, err := repo_model.GetRepoWatchersIDs(ctx, rel.RepoID)
	if err != nil {
		log.Error("GetRepoWatchersIDs: %v", err)
		return
	}

	if err := rel.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}
	if err := rel.Repo.LoadOwner(ctx); err != nil {
		log.Error("LoadOwner: %v", err)
		return
	}
	if !rel.Repo.Owner.IsOrganization() && !slices.Contains(repoWatcherIDs, rel.Repo.Owner.ID) && rel.Repo.Owner.ID != doer.ID {
		repoWatcherIDs = append(repoWatcherIDs, rel.Repo.Owner.ID)
	}

	// Watching a repository does not imply still being able to read it: a watcher can lose
	// access after the fact, so the release title must not leak to them.
	watcherIDs, err := filterRecipientsByRepoAccess(ctx, rel.Repo, repoWatcherIDs, unit.TypeReleases)
	if err != nil {
		log.Error("filterRecipientsByRepoAccess: %v", err)
		return
	}

	for _, watcherID := range watcherIDs {
		if watcherID == doer.ID {
			// Do not notify the publisher of the release
			continue
		}

		opts.ReceiverID = watcherID
		_ = ns.queue.Push(opts)
	}
}
