// Copyright 2018 The Gitea Authors. All rights reserved.
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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
)

var notifiers []Notifier

// RegisterNotifier providers method to receive notify messages
func RegisterNotifier(notifier Notifier) {
	go notifier.Run()
	notifiers = append(notifiers, notifier)
}

// NewWikiPage notifies creating new wiki pages to notifiers
func NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	for _, notifier := range notifiers {
		notifier.NewWikiPage(ctx, doer, repo, page, comment)
	}
}

// EditWikiPage notifies editing or renaming wiki pages to notifiers
func EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string) {
	for _, notifier := range notifiers {
		notifier.EditWikiPage(ctx, doer, repo, page, comment)
	}
}

// DeleteWikiPage notifies deleting wiki pages to notifiers
func DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string) {
	for _, notifier := range notifiers {
		notifier.DeleteWikiPage(ctx, doer, repo, page)
	}
}

// CreateIssueComment notifies issue comment related message to notifiers
func CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	for _, notifier := range notifiers {
		notifier.CreateIssueComment(ctx, doer, repo, issue, comment, mentions)
	}
}

// NewIssue notifies new issue to notifiers
func NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NewIssue(ctx, issue, mentions)
	}
}

// IssueChangeStatus notifies close or reopen issue to notifiers
func IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool) {
	for _, notifier := range notifiers {
		notifier.IssueChangeStatus(ctx, doer, commitID, issue, actionComment, closeOrReopen)
	}
}

// DeleteIssue notify when some issue deleted
func DeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.DeleteIssue(ctx, doer, issue)
	}
}

// MergePullRequest notifies merge pull request to notifiers
func MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.MergePullRequest(ctx, doer, pr)
	}
}

// AutoMergePullRequest notifies merge pull request to notifiers
func AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.AutoMergePullRequest(ctx, doer, pr)
	}
}

// NewPullRequest notifies new pull request to notifiers
func NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue failed: %v", err)
		return
	}
	if err := pr.Issue.LoadPoster(ctx); err != nil {
		return
	}
	for _, notifier := range notifiers {
		notifier.NewPullRequest(ctx, pr, mentions)
	}
}

// PullRequestSynchronized notifies Synchronized pull request
func PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.PullRequestSynchronized(ctx, doer, pr)
	}
}

// PullRequestReview notifies new pull request review
func PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("LoadReviewer failed: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.PullRequestReview(ctx, pr, review, comment, mentions)
	}
}

// PullRequestCodeComment notifies new pull request code comment
func PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := comment.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.PullRequestCodeComment(ctx, pr, comment, mentions)
	}
}

// PullRequestChangeTargetBranch notifies when a pull request's target branch was changed
func PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	for _, notifier := range notifiers {
		notifier.PullRequestChangeTargetBranch(ctx, doer, pr, oldBranch)
	}
}

// PullRequestPushCommits notifies when push commits to pull request's head branch
func PullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.PullRequestPushCommits(ctx, doer, pr, comment)
	}
}

// PullReviewDismiss notifies when a review was dismissed by repo admin
func PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.PullReviewDismiss(ctx, doer, review, comment)
	}
}

// UpdateComment notifies update comment to notifiers
func UpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string) {
	for _, notifier := range notifiers {
		notifier.UpdateComment(ctx, doer, c, oldContent)
	}
}

// DeleteComment notifies delete comment to notifiers
func DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.DeleteComment(ctx, doer, c)
	}
}

// NewRelease notifies new release to notifiers
func NewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadPublisher: %v", err)
		return
	}
	for _, notifier := range notifiers {
		notifier.NewRelease(ctx, rel)
	}
}

// UpdateRelease notifies update release to notifiers
func UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	for _, notifier := range notifiers {
		notifier.UpdateRelease(ctx, doer, rel)
	}
}

// DeleteRelease notifies delete release to notifiers
func DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release) {
	for _, notifier := range notifiers {
		notifier.DeleteRelease(ctx, doer, rel)
	}
}

// IssueChangeMilestone notifies change milestone to notifiers
func IssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	for _, notifier := range notifiers {
		notifier.IssueChangeMilestone(ctx, doer, issue, oldMilestoneID)
	}
}

// IssueChangeContent notifies change content to notifiers
func IssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	for _, notifier := range notifiers {
		notifier.IssueChangeContent(ctx, doer, issue, oldContent)
	}
}

// IssueChangeAssignee notifies change content to notifiers
func IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.IssueChangeAssignee(ctx, doer, issue, assignee, removed, comment)
	}
}

// PullRequestReviewRequest notifies Request Review change
func PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.PullRequestReviewRequest(ctx, doer, issue, reviewer, isRequest, comment)
	}
}

// IssueClearLabels notifies clear labels to notifiers
func IssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.IssueClearLabels(ctx, doer, issue)
	}
}

// IssueChangeTitle notifies change title to notifiers
func IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	for _, notifier := range notifiers {
		notifier.IssueChangeTitle(ctx, doer, issue, oldTitle)
	}
}

// IssueChangeRef notifies change reference to notifiers
func IssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldRef string) {
	for _, notifier := range notifiers {
		notifier.IssueChangeRef(ctx, doer, issue, oldRef)
	}
}

// IssueChangeLabels notifies change labels to notifiers
func IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label,
) {
	for _, notifier := range notifiers {
		notifier.IssueChangeLabels(ctx, doer, issue, addedLabels, removedLabels)
	}
}

// CreateRepository notifies create repository to notifiers
func CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.CreateRepository(ctx, doer, u, repo)
	}
}

// AdoptRepository notifies the adoption of a repository to notifiers
func AdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.AdoptRepository(ctx, doer, u, repo)
	}
}

// MigrateRepository notifies create repository to notifiers
func MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.MigrateRepository(ctx, doer, u, repo)
	}
}

// TransferRepository notifies create repository to notifiers
func TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	for _, notifier := range notifiers {
		notifier.TransferRepository(ctx, doer, repo, newOwnerName)
	}
}

// DeleteRepository notifies delete repository to notifiers
func DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.DeleteRepository(ctx, doer, repo)
	}
}

// ForkRepository notifies fork repository to notifiers
func ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.ForkRepository(ctx, doer, oldRepo, repo)
	}
}

// RenameRepository notifies repository renamed
func RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldName string) {
	for _, notifier := range notifiers {
		notifier.RenameRepository(ctx, doer, repo, oldName)
	}
}

// PushCommits notifies commits pushed to notifiers
func PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.PushCommits(ctx, pusher, repo, opts, commits)
	}
}

// CreateRef notifies branch or tag creation to notifiers
func CreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	for _, notifier := range notifiers {
		notifier.CreateRef(ctx, pusher, repo, refFullName, refID)
	}
}

// DeleteRef notifies branch or tag deletion to notifiers
func DeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	for _, notifier := range notifiers {
		notifier.DeleteRef(ctx, pusher, repo, refFullName)
	}
}

// SyncPushCommits notifies commits pushed to notifiers
func SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.SyncPushCommits(ctx, pusher, repo, opts, commits)
	}
}

// SyncCreateRef notifies branch or tag creation to notifiers
func SyncCreateRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	for _, notifier := range notifiers {
		notifier.SyncCreateRef(ctx, pusher, repo, refFullName, refID)
	}
}

// SyncDeleteRef notifies branch or tag deletion to notifiers
func SyncDeleteRef(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	for _, notifier := range notifiers {
		notifier.SyncDeleteRef(ctx, pusher, repo, refFullName)
	}
}

// RepoPendingTransfer notifies creation of pending transfer to notifiers
func RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.RepoPendingTransfer(ctx, doer, newOwner, repo)
	}
}

// PackageCreate notifies creation of a package to notifiers
func PackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.PackageCreate(ctx, doer, pd)
	}
}

// PackageDelete notifies deletion of a package to notifiers
func PackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.PackageDelete(ctx, doer, pd)
	}
}

// ChangeDefaultBranch notifies change default branch to notifiers
func ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.ChangeDefaultBranch(ctx, repo)
	}
}

func CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
	for _, notifier := range notifiers {
		notifier.CreateCommitStatus(ctx, repo, commit, sender, status)
	}
}
