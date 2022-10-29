// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/repository"
)

// NullNotifier implements a blank notifier
type NullNotifier struct{}

var _ Notifier = &NullNotifier{}

// Run places a place holder function
func (*NullNotifier) Run() {
}

// NotifyCreateIssueComment places a place holder function
func (*NullNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyNewIssue places a place holder function
func (*NullNotifier) NotifyNewIssue(issue *issues_model.Issue, mentions []*user_model.User) {
}

// NotifyIssueChangeStatus places a place holder function
func (*NullNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
}

// NotifyDeleteIssue notify when some issue deleted
func (*NullNotifier) NotifyDeleteIssue(doer *user_model.User, issue *issues_model.Issue) {
}

// NotifyNewPullRequest places a place holder function
func (*NullNotifier) NotifyNewPullRequest(pr *issues_model.PullRequest, mentions []*user_model.User) {
}

// NotifyPullRequestReview places a place holder function
func (*NullNotifier) NotifyPullRequestReview(pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyPullRequestCodeComment places a place holder function
func (*NullNotifier) NotifyPullRequestCodeComment(pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NotifyMergePullRequest places a place holder function
func (*NullNotifier) NotifyMergePullRequest(pr *issues_model.PullRequest, doer *user_model.User) {
}

// NotifyAutoMergePullRequest places a place holder function
func (*NullNotifier) NotifyAutoMergePullRequest(pr *issues_model.PullRequest, doer *user_model.User) {
}

// NotifyPullRequestSynchronized places a place holder function
func (*NullNotifier) NotifyPullRequestSynchronized(doer *user_model.User, pr *issues_model.PullRequest) {
}

// NotifyPullRequestChangeTargetBranch places a place holder function
func (*NullNotifier) NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func (*NullNotifier) NotifyPullRequestPushCommits(doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
}

// NotifyPullRevieweDismiss notifies when a review was dismissed by repo admin
func (*NullNotifier) NotifyPullRevieweDismiss(doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
}

// NotifyUpdateComment places a place holder function
func (*NullNotifier) NotifyUpdateComment(doer *user_model.User, c *issues_model.Comment, oldContent string) {
}

// NotifyDeleteComment places a place holder function
func (*NullNotifier) NotifyDeleteComment(doer *user_model.User, c *issues_model.Comment) {
}

// NotifyNewWikiPage places a place holder function
func (*NullNotifier) NotifyNewWikiPage(doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// NotifyEditWikiPage places a place holder function
func (*NullNotifier) NotifyEditWikiPage(doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// NotifyDeleteWikiPage places a place holder function
func (*NullNotifier) NotifyDeleteWikiPage(doer *user_model.User, repo *repo_model.Repository, page string) {
}

// NotifyNewRelease places a place holder function
func (*NullNotifier) NotifyNewRelease(rel *repo_model.Release) {
}

// NotifyUpdateRelease places a place holder function
func (*NullNotifier) NotifyUpdateRelease(doer *user_model.User, rel *repo_model.Release) {
}

// NotifyDeleteRelease places a place holder function
func (*NullNotifier) NotifyDeleteRelease(doer *user_model.User, rel *repo_model.Release) {
}

// NotifyIssueChangeMilestone places a place holder function
func (*NullNotifier) NotifyIssueChangeMilestone(doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
}

// NotifyIssueChangeContent places a place holder function
func (*NullNotifier) NotifyIssueChangeContent(doer *user_model.User, issue *issues_model.Issue, oldContent string) {
}

// NotifyIssueChangeAssignee places a place holder function
func (*NullNotifier) NotifyIssueChangeAssignee(doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
}

// NotifyPullReviewRequest places a place holder function
func (*NullNotifier) NotifyPullReviewRequest(doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
}

// NotifyIssueClearLabels places a place holder function
func (*NullNotifier) NotifyIssueClearLabels(doer *user_model.User, issue *issues_model.Issue) {
}

// NotifyIssueChangeTitle places a place holder function
func (*NullNotifier) NotifyIssueChangeTitle(doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// NotifyIssueChangeRef places a place holder function
func (*NullNotifier) NotifyIssueChangeRef(doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// NotifyIssueChangeLabels places a place holder function
func (*NullNotifier) NotifyIssueChangeLabels(doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label) {
}

// NotifyCreateRepository places a place holder function
func (*NullNotifier) NotifyCreateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
}

// NotifyDeleteRepository places a place holder function
func (*NullNotifier) NotifyDeleteRepository(doer *user_model.User, repo *repo_model.Repository) {
}

// NotifyForkRepository places a place holder function
func (*NullNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
}

// NotifyMigrateRepository places a place holder function
func (*NullNotifier) NotifyMigrateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
}

// NotifyPushCommits notifies commits pushed to notifiers
func (*NullNotifier) NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func (*NullNotifier) NotifyCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func (*NullNotifier) NotifyDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

// NotifyRenameRepository places a place holder function
func (*NullNotifier) NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

// NotifyTransferRepository places a place holder function
func (*NullNotifier) NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

// NotifySyncPushCommits places a place holder function
func (*NullNotifier) NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifySyncCreateRef places a place holder function
func (*NullNotifier) NotifySyncCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
}

// NotifySyncDeleteRef places a place holder function
func (*NullNotifier) NotifySyncDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
}

// NotifyRepoPendingTransfer places a place holder function
func (*NullNotifier) NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *repo_model.Repository) {
}

// NotifyPackageCreate places a place holder function
func (*NullNotifier) NotifyPackageCreate(doer *user_model.User, pd *packages_model.PackageDescriptor) {
}

// NotifyPackageDelete places a place holder function
func (*NullNotifier) NotifyPackageDelete(doer *user_model.User, pd *packages_model.PackageDescriptor) {
}
