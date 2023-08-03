// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/repository"
)

// NullNotifier implements a blank notifier
type NullNotifier struct{}

var _ Notifier = &NullNotifier{}

// Run places a place holder function
func (*NullNotifier) Run() {
}

// NotifyCreateIssueComment places a place holder function
func (*NullNotifier) NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyNewIssue places a place holder function
func (*NullNotifier) NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
}

// NotifyIssueChangeStatus places a place holder function
func (*NullNotifier) NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
}

// NotifyDeleteIssue notify when some issue deleted
func (*NullNotifier) NotifyDeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
}

// NotifyNewPullRequest places a place holder function
func (*NullNotifier) NotifyNewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
}

// NotifyPullRequestReview places a place holder function
func (*NullNotifier) NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyPullRequestCodeComment places a place holder function
func (*NullNotifier) NotifyPullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyMergePullRequest places a place holder function
func (*NullNotifier) NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// NotifyAutoMergePullRequest places a place holder function
func (*NullNotifier) NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// NotifyPullRequestSynchronized places a place holder function
func (*NullNotifier) NotifyPullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// NotifyPullRequestChangeTargetBranch places a place holder function
func (*NullNotifier) NotifyPullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func (*NullNotifier) NotifyPullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
}

// NotifyPullReviewDismiss notifies when a review was dismissed by repo admin
func (*NullNotifier) NotifyPullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
}

// NotifyUpdateComment places a place holder function
func (*NullNotifier) NotifyUpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
}

// NotifyDeleteComment places a place holder function
func (*NullNotifier) NotifyDeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
}

// NotifyNewWikiPage places a place holder function
func (*NullNotifier) NotifyNewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// NotifyEditWikiPage places a place holder function
func (*NullNotifier) NotifyEditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// NotifyDeleteWikiPage places a place holder function
func (*NullNotifier) NotifyDeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
}

// NotifyNewRelease places a place holder function
func (*NullNotifier) NotifyNewRelease(ctx context.Context, rel *repo_model.Release) {
}

// NotifyUpdateRelease places a place holder function
func (*NullNotifier) NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
}

// NotifyDeleteRelease places a place holder function
func (*NullNotifier) NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
}

// NotifyIssueChangeMilestone places a place holder function
func (*NullNotifier) NotifyIssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
}

// NotifyIssueChangeContent places a place holder function
func (*NullNotifier) NotifyIssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
}

// NotifyIssueChangeAssignee places a place holder function
func (*NullNotifier) NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
}

// NotifyPullRequestReviewRequest places a place holder function
func (*NullNotifier) NotifyPullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
}

// NotifyIssueClearLabels places a place holder function
func (*NullNotifier) NotifyIssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
}

// NotifyIssueChangeTitle places a place holder function
func (*NullNotifier) NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// NotifyIssueChangeRef places a place holder function
func (*NullNotifier) NotifyIssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// NotifyIssueChangeLabels places a place holder function
func (*NullNotifier) NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label) {
}

// NotifyCreateRepository places a place holder function
func (*NullNotifier) NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// NotifyAdoptRepository places a place holder function
func (*NullNotifier) NotifyAdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// NotifyDeleteRepository places a place holder function
func (*NullNotifier) NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
}

// NotifyForkRepository places a place holder function
func (*NullNotifier) NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
}

// NotifyMigrateRepository places a place holder function
func (*NullNotifier) NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// NotifyPushCommits notifies commits pushed to notifiers
func (*NullNotifier) NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func (*NullNotifier) NotifyCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func (*NullNotifier) NotifyDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
}

// NotifyRenameRepository places a place holder function
func (*NullNotifier) NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

// NotifyTransferRepository places a place holder function
func (*NullNotifier) NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

// NotifySyncPushCommits places a place holder function
func (*NullNotifier) NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifySyncCreateRef places a place holder function
func (*NullNotifier) NotifySyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
}

// NotifySyncDeleteRef places a place holder function
func (*NullNotifier) NotifySyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
}

// NotifyRepoPendingTransfer places a place holder function
func (*NullNotifier) NotifyRepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
}

// NotifyPackageCreate places a place holder function
func (*NullNotifier) NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
}

// NotifyPackageDelete places a place holder function
func (*NullNotifier) NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
}
