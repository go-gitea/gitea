// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/repository"
)

// NullNotifier implements a blank notifier
type NullNotifier struct {
}

var (
	_ Notifier = &NullNotifier{}
)

// Run places a place holder function
func (*NullNotifier) Run() {
}

// NotifyCreateIssueComment places a place holder function
func (*NullNotifier) NotifyCreateIssueComment(doer *user_model.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*user_model.User) {
}

// NotifyNewIssue places a place holder function
func (*NullNotifier) NotifyNewIssue(issue *models.Issue, mentions []*user_model.User) {
}

// NotifyIssueChangeStatus places a place holder function
func (*NullNotifier) NotifyIssueChangeStatus(doer *user_model.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
}

// NotifyNewPullRequest places a place holder function
func (*NullNotifier) NotifyNewPullRequest(pr *models.PullRequest, mentions []*user_model.User) {
}

// NotifyPullRequestReview places a place holder function
func (*NullNotifier) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, comment *models.Comment, mentions []*user_model.User) {
}

// NotifyPullRequestCodeComment places a place holder function
func (*NullNotifier) NotifyPullRequestCodeComment(pr *models.PullRequest, comment *models.Comment, mentions []*user_model.User) {
}

// NotifyMergePullRequest places a place holder function
func (*NullNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *user_model.User) {
}

// NotifyPullRequestSynchronized places a place holder function
func (*NullNotifier) NotifyPullRequestSynchronized(doer *user_model.User, pr *models.PullRequest) {
}

// NotifyPullRequestChangeTargetBranch places a place holder function
func (*NullNotifier) NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *models.PullRequest, oldBranch string) {
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func (*NullNotifier) NotifyPullRequestPushCommits(doer *user_model.User, pr *models.PullRequest, comment *models.Comment) {
}

// NotifyPullRevieweDismiss notifies when a review was dismissed by repo admin
func (*NullNotifier) NotifyPullRevieweDismiss(doer *user_model.User, review *models.Review, comment *models.Comment) {
}

// NotifyUpdateComment places a place holder function
func (*NullNotifier) NotifyUpdateComment(doer *user_model.User, c *models.Comment, oldContent string) {
}

// NotifyDeleteComment places a place holder function
func (*NullNotifier) NotifyDeleteComment(doer *user_model.User, c *models.Comment) {
}

// NotifyNewRelease places a place holder function
func (*NullNotifier) NotifyNewRelease(rel *models.Release) {
}

// NotifyUpdateRelease places a place holder function
func (*NullNotifier) NotifyUpdateRelease(doer *user_model.User, rel *models.Release) {
}

// NotifyDeleteRelease places a place holder function
func (*NullNotifier) NotifyDeleteRelease(doer *user_model.User, rel *models.Release) {
}

// NotifyIssueChangeMilestone places a place holder function
func (*NullNotifier) NotifyIssueChangeMilestone(doer *user_model.User, issue *models.Issue, oldMilestoneID int64) {
}

// NotifyIssueChangeContent places a place holder function
func (*NullNotifier) NotifyIssueChangeContent(doer *user_model.User, issue *models.Issue, oldContent string) {
}

// NotifyIssueChangeAssignee places a place holder function
func (*NullNotifier) NotifyIssueChangeAssignee(doer *user_model.User, issue *models.Issue, assignee *user_model.User, removed bool, comment *models.Comment) {
}

// NotifyPullReviewRequest places a place holder function
func (*NullNotifier) NotifyPullReviewRequest(doer *user_model.User, issue *models.Issue, reviewer *user_model.User, isRequest bool, comment *models.Comment) {
}

// NotifyIssueClearLabels places a place holder function
func (*NullNotifier) NotifyIssueClearLabels(doer *user_model.User, issue *models.Issue) {
}

// NotifyIssueChangeTitle places a place holder function
func (*NullNotifier) NotifyIssueChangeTitle(doer *user_model.User, issue *models.Issue, oldTitle string) {
}

// NotifyIssueChangeRef places a place holder function
func (*NullNotifier) NotifyIssueChangeRef(doer *user_model.User, issue *models.Issue, oldTitle string) {
}

// NotifyIssueChangeLabels places a place holder function
func (*NullNotifier) NotifyIssueChangeLabels(doer *user_model.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
}

// NotifyCreateRepository places a place holder function
func (*NullNotifier) NotifyCreateRepository(doer *user_model.User, u *user_model.User, repo *models.Repository) {
}

// NotifyDeleteRepository places a place holder function
func (*NullNotifier) NotifyDeleteRepository(doer *user_model.User, repo *models.Repository) {
}

// NotifyForkRepository places a place holder function
func (*NullNotifier) NotifyForkRepository(doer *user_model.User, oldRepo, repo *models.Repository) {
}

// NotifyMigrateRepository places a place holder function
func (*NullNotifier) NotifyMigrateRepository(doer *user_model.User, u *user_model.User, repo *models.Repository) {
}

// NotifyPushCommits notifies commits pushed to notifiers
func (*NullNotifier) NotifyPushCommits(pusher *user_model.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func (*NullNotifier) NotifyCreateRef(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func (*NullNotifier) NotifyDeleteRef(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyRenameRepository places a place holder function
func (*NullNotifier) NotifyRenameRepository(doer *user_model.User, repo *models.Repository, oldRepoName string) {
}

// NotifyTransferRepository places a place holder function
func (*NullNotifier) NotifyTransferRepository(doer *user_model.User, repo *models.Repository, oldOwnerName string) {
}

// NotifySyncPushCommits places a place holder function
func (*NullNotifier) NotifySyncPushCommits(pusher *user_model.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifySyncCreateRef places a place holder function
func (*NullNotifier) NotifySyncCreateRef(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifySyncDeleteRef places a place holder function
func (*NullNotifier) NotifySyncDeleteRef(doer *user_model.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyRepoPendingTransfer places a place holder function
func (*NullNotifier) NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *models.Repository) {
}
