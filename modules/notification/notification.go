// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notification

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/action"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/notification/indexer"
	"code.gitea.io/gitea/modules/notification/mail"
	"code.gitea.io/gitea/modules/notification/mirror"
	"code.gitea.io/gitea/modules/notification/ui"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
)

var notifiers []base.Notifier

// RegisterNotifier providers method to receive notify messages
func RegisterNotifier(notifier base.Notifier) {
	go notifier.Run()
	notifiers = append(notifiers, notifier)
}

// NewContext registers notification handlers
func NewContext() {
	RegisterNotifier(ui.NewNotifier())
	if setting.Service.EnableNotifyMail {
		RegisterNotifier(mail.NewNotifier())
	}
	RegisterNotifier(indexer.NewNotifier())
	RegisterNotifier(action.NewNotifier())
	RegisterNotifier(mirror.NewNotifier())
}

// NotifyNewWikiPage notifies creating new wiki pages to notifiers
func NotifyNewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	for _, notifier := range notifiers {
		notifier.NotifyNewWikiPage(ctx, doer, repo, page, comment)
	}
}

// NotifyEditWikiPage notifies editing or renaming wiki pages to notifiers
func NotifyEditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	for _, notifier := range notifiers {
		notifier.NotifyEditWikiPage(ctx, doer, repo, page, comment)
	}
}

// NotifyDeleteWikiPage notifies deleting wiki pages to notifiers
func NotifyDeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteWikiPage(ctx, doer, repo, page)
	}
}

// NotifyCreateIssueComment notifies issue comment related message to notifiers
func NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateIssueComment(ctx, doer, repo, issue, comment, mentions)
	}
}

// NotifyNewIssue notifies new issue to notifiers
func NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyNewIssue(ctx, issue, mentions)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeStatus(ctx, doer, commitID, issue, actionComment, closeOrReopen)
	}
}

// NotifyDeleteIssue notify when some issue deleted
func NotifyDeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteIssue(ctx, doer, issue)
	}
}

// NotifyMergePullRequest notifies merge pull request to notifiers
func NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyMergePullRequest(ctx, doer, pr)
	}
}

// NotifyAutoMergePullRequest notifies merge pull request to notifiers
func NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyAutoMergePullRequest(ctx, doer, pr)
	}
}

// NotifyNewPullRequest notifies new pull request to notifiers
func NotifyNewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("%v", err)
		return
	}
	if err := pr.Issue.LoadPoster(ctx); err != nil {
		return
	}
	for _, notifier := range notifiers {
		notifier.NotifyNewPullRequest(ctx, pr, mentions)
	}
}

// NotifyPullRequestSynchronized notifies Synchronized pull request
func NotifyPullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestSynchronized(ctx, doer, pr)
	}
}

// NotifyPullRequestReview notifies new pull request review
func NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("%v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestReview(ctx, pr, review, comment, mentions)
	}
}

// NotifyPullRequestCodeComment notifies new pull request code comment
func NotifyPullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := comment.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestCodeComment(ctx, pr, comment, mentions)
	}
}

// NotifyPullRequestChangeTargetBranch notifies when a pull request's target branch was changed
func NotifyPullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestChangeTargetBranch(ctx, doer, pr, oldBranch)
	}
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func NotifyPullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestPushCommits(ctx, doer, pr, comment)
	}
}

// NotifyPullReviewDismiss notifies when a review was dismissed by repo admin
func NotifyPullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullReviewDismiss(ctx, doer, review, comment)
	}
}

// NotifyUpdateComment notifies update comment to notifiers
func NotifyUpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateComment(ctx, doer, c, oldContent)
	}
}

// NotifyDeleteComment notifies delete comment to notifiers
func NotifyDeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteComment(ctx, doer, c)
	}
}

// NotifyNewRelease notifies new release to notifiers
func NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadPublisher: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.NotifyNewRelease(ctx, rel)
	}
}

// NotifyUpdateRelease notifies update release to notifiers
func NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateRelease(ctx, doer, rel)
	}
}

// NotifyDeleteRelease notifies delete release to notifiers
func NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRelease(ctx, doer, rel)
	}
}

// NotifyIssueChangeMilestone notifies change milestone to notifiers
func NotifyIssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeMilestone(ctx, doer, issue, oldMilestoneID)
	}
}

// NotifyIssueChangeContent notifies change content to notifiers
func NotifyIssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeContent(ctx, doer, issue, oldContent)
	}
}

// NotifyIssueChangeAssignee notifies change content to notifiers
func NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeAssignee(ctx, doer, issue, assignee, removed, comment)
	}
}

// NotifyPullRequestReviewRequest notifies Request Review change
func NotifyPullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestReviewRequest(ctx, doer, issue, reviewer, isRequest, comment)
	}
}

// NotifyIssueClearLabels notifies clear labels to notifiers
func NotifyIssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueClearLabels(ctx, doer, issue)
	}
}

// NotifyIssueChangeTitle notifies change title to notifiers
func NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeTitle(ctx, doer, issue, oldTitle)
	}
}

// NotifyIssueChangeRef notifies change reference to notifiers
func NotifyIssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldRef string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeRef(ctx, doer, issue, oldRef)
	}
}

// NotifyIssueChangeLabels notifies change labels to notifiers
func NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label,
) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeLabels(ctx, doer, issue, addedLabels, removedLabels)
	}
}

// NotifyCreateRepository notifies create repository to notifiers
func NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRepository(ctx, doer, u, repo)
	}
}

// NotifyAdoptRepository notifies the adoption of a repository to notifiers
func NotifyAdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyAdoptRepository(ctx, doer, u, repo)
	}
}

// NotifyMigrateRepository notifies create repository to notifiers
func NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyMigrateRepository(ctx, doer, u, repo)
	}
}

// NotifyTransferRepository notifies create repository to notifiers
func NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	for _, notifier := range notifiers {
		notifier.NotifyTransferRepository(ctx, doer, repo, newOwnerName)
	}
}

// NotifyDeleteRepository notifies delete repository to notifiers
func NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRepository(ctx, doer, repo)
	}
}

// NotifyForkRepository notifies fork repository to notifiers
func NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyForkRepository(ctx, doer, oldRepo, repo)
	}
}

// NotifyRenameRepository notifies repository renamed
func NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	for _, notifier := range notifiers {
		notifier.NotifyRenameRepository(ctx, doer, repo, oldName)
	}
}

// NotifyPushCommits notifies commits pushed to notifiers
func NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifyPushCommits(ctx, pusher, repo, opts, commits)
	}
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func NotifyCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRef(ctx, pusher, repo, refFullName, refID)
	}
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func NotifyDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRef(ctx, pusher, repo, refFullName)
	}
}

// NotifySyncPushCommits notifies commits pushed to notifiers
func NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifySyncPushCommits(ctx, pusher, repo, opts, commits)
	}
}

// NotifySyncCreateRef notifies branch or tag creation to notifiers
func NotifySyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	for _, notifier := range notifiers {
		notifier.NotifySyncCreateRef(ctx, pusher, repo, refFullName, refID)
	}
}

// NotifySyncDeleteRef notifies branch or tag deletion to notifiers
func NotifySyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	for _, notifier := range notifiers {
		notifier.NotifySyncDeleteRef(ctx, pusher, repo, refFullName)
	}
}

// NotifyRepoPendingTransfer notifies creation of pending transfer to notifiers
func NotifyRepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyRepoPendingTransfer(ctx, doer, newOwner, repo)
	}
}

// NotifyPackageCreate notifies creation of a package to notifiers
func NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.NotifyPackageCreate(ctx, doer, pd)
	}
}

// NotifyPackageDelete notifies deletion of a package to notifiers
func NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.NotifyPackageDelete(ctx, doer, pd)
	}
}
