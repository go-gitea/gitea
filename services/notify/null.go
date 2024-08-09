// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package notify

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
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

// CreateIssueComment places a place holder function
func (*NullNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
}

// NewIssue places a place holder function
func (*NullNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
}

// IssueChangeStatus places a place holder function
func (*NullNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
}

// DeleteIssue notify when some issue deleted
func (*NullNotifier) DeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
}

// NewPullRequest places a place holder function
func (*NullNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
}

// PullRequestReview places a place holder function
func (*NullNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, r *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
}

// PullRequestCodeComment places a place holder function
func (*NullNotifier) PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
}

// MergePullRequest places a place holder function
func (*NullNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// AutoMergePullRequest places a place holder function
func (*NullNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// PullRequestSynchronized places a place holder function
func (*NullNotifier) PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
}

// PullRequestChangeTargetBranch places a place holder function
func (*NullNotifier) PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
}

// PullRequestPushCommits notifies when push commits to pull request's head branch
func (*NullNotifier) PullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
}

// PullReviewDismiss notifies when a review was dismissed by repo admin
func (*NullNotifier) PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
}

// UpdateComment places a place holder function
func (*NullNotifier) UpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
}

// DeleteComment places a place holder function
func (*NullNotifier) DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
}

// NewWikiPage places a place holder function
func (*NullNotifier) NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// EditWikiPage places a place holder function
func (*NullNotifier) EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
}

// DeleteWikiPage places a place holder function
func (*NullNotifier) DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
}

// NewRelease places a place holder function
func (*NullNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
}

// UpdateRelease places a place holder function
func (*NullNotifier) UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
}

// DeleteRelease places a place holder function
func (*NullNotifier) DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
}

// IssueChangeMilestone places a place holder function
func (*NullNotifier) IssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
}

// IssueChangeContent places a place holder function
func (*NullNotifier) IssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
}

// IssueChangeAssignee places a place holder function
func (*NullNotifier) IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
}

// PullRequestReviewRequest places a place holder function
func (*NullNotifier) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
}

// IssueClearLabels places a place holder function
func (*NullNotifier) IssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
}

// IssueChangeTitle places a place holder function
func (*NullNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// IssueChangeRef places a place holder function
func (*NullNotifier) IssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
}

// IssueChangeLabels places a place holder function
func (*NullNotifier) IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label) {
}

// CreateRepository places a place holder function
func (*NullNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// AdoptRepository places a place holder function
func (*NullNotifier) AdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// DeleteRepository places a place holder function
func (*NullNotifier) DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
}

// ForkRepository places a place holder function
func (*NullNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
}

// MigrateRepository places a place holder function
func (*NullNotifier) MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
}

// PushCommits notifies commits pushed to notifiers
func (*NullNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// CreateRef notifies branch or tag creation to notifiers
func (*NullNotifier) CreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
}

// DeleteRef notifies branch or tag deletion to notifiers
func (*NullNotifier) DeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
}

// RenameRepository places a place holder function
func (*NullNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
}

// TransferRepository places a place holder function
func (*NullNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
}

// SyncPushCommits places a place holder function
func (*NullNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// SyncCreateRef places a place holder function
func (*NullNotifier) SyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
}

// SyncDeleteRef places a place holder function
func (*NullNotifier) SyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
}

// RepoPendingTransfer places a place holder function
func (*NullNotifier) RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
}

// PackageCreate places a place holder function
func (*NullNotifier) PackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
}

// PackageDelete places a place holder function
func (*NullNotifier) PackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
}

// ChangeDefaultBranch places a place holder function
func (*NullNotifier) ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
}

func (*NullNotifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
}
