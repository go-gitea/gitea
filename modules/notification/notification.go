// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notification

import (
	"code.gitea.io/gitea/models"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification/action"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/notification/indexer"
	"code.gitea.io/gitea/modules/notification/mail"
	"code.gitea.io/gitea/modules/notification/mirror"
	"code.gitea.io/gitea/modules/notification/ui"
	"code.gitea.io/gitea/modules/notification/webhook"
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
	RegisterNotifier(webhook.NewNotifier())
	RegisterNotifier(action.NewNotifier())
	RegisterNotifier(mirror.NewNotifier())
}

// NotifyCreateIssueComment notifies issue comment related message to notifiers
func NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateIssueComment(doer, repo, issue, comment, mentions)
	}
}

// NotifyNewIssue notifies new issue to notifiers
func NotifyNewIssue(issue *issues_model.Issue, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyNewIssue(issue, mentions)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func NotifyIssueChangeStatus(doer *user_model.User, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeStatus(doer, issue, actionComment, closeOrReopen)
	}
}

// NotifyDeleteIssue notify when some issue deleted
func NotifyDeleteIssue(doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteIssue(doer, issue)
	}
}

// NotifyMergePullRequest notifies merge pull request to notifiers
func NotifyMergePullRequest(pr *issues_model.PullRequest, doer *user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyMergePullRequest(pr, doer)
	}
}

// NotifyNewPullRequest notifies new pull request to notifiers
func NotifyNewPullRequest(pr *issues_model.PullRequest, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyNewPullRequest(pr, mentions)
	}
}

// NotifyPullRequestSynchronized notifies Synchronized pull request
func NotifyPullRequestSynchronized(doer *user_model.User, pr *issues_model.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestSynchronized(doer, pr)
	}
}

// NotifyPullRequestReview notifies new pull request review
func NotifyPullRequestReview(pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestReview(pr, review, comment, mentions)
	}
}

// NotifyPullRequestCodeComment notifies new pull request code comment
func NotifyPullRequestCodeComment(pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestCodeComment(pr, comment, mentions)
	}
}

// NotifyPullRequestChangeTargetBranch notifies when a pull request's target branch was changed
func NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *issues_model.PullRequest, oldBranch string) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestChangeTargetBranch(doer, pr, oldBranch)
	}
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func NotifyPullRequestPushCommits(doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestPushCommits(doer, pr, comment)
	}
}

// NotifyPullRevieweDismiss notifies when a review was dismissed by repo admin
func NotifyPullRevieweDismiss(doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRevieweDismiss(doer, review, comment)
	}
}

// NotifyUpdateComment notifies update comment to notifiers
func NotifyUpdateComment(doer *user_model.User, c *issues_model.Comment, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateComment(doer, c, oldContent)
	}
}

// NotifyDeleteComment notifies delete comment to notifiers
func NotifyDeleteComment(doer *user_model.User, c *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteComment(doer, c)
	}
}

// NotifyNewRelease notifies new release to notifiers
func NotifyNewRelease(rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyNewRelease(rel)
	}
}

// NotifyUpdateRelease notifies update release to notifiers
func NotifyUpdateRelease(doer *user_model.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateRelease(doer, rel)
	}
}

// NotifyDeleteRelease notifies delete release to notifiers
func NotifyDeleteRelease(doer *user_model.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRelease(doer, rel)
	}
}

// NotifyIssueChangeMilestone notifies change milestone to notifiers
func NotifyIssueChangeMilestone(doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeMilestone(doer, issue, oldMilestoneID)
	}
}

// NotifyIssueChangeContent notifies change content to notifiers
func NotifyIssueChangeContent(doer *user_model.User, issue *issues_model.Issue, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeContent(doer, issue, oldContent)
	}
}

// NotifyIssueChangeAssignee notifies change content to notifiers
func NotifyIssueChangeAssignee(doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeAssignee(doer, issue, assignee, removed, comment)
	}
}

// NotifyPullReviewRequest notifies Request Review change
func NotifyPullReviewRequest(doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullReviewRequest(doer, issue, reviewer, isRequest, comment)
	}
}

// NotifyIssueClearLabels notifies clear labels to notifiers
func NotifyIssueClearLabels(doer *user_model.User, issue *issues_model.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueClearLabels(doer, issue)
	}
}

// NotifyIssueChangeTitle notifies change title to notifiers
func NotifyIssueChangeTitle(doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeTitle(doer, issue, oldTitle)
	}
}

// NotifyIssueChangeRef notifies change reference to notifiers
func NotifyIssueChangeRef(doer *user_model.User, issue *issues_model.Issue, oldRef string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeRef(doer, issue, oldRef)
	}
}

// NotifyIssueChangeLabels notifies change labels to notifiers
func NotifyIssueChangeLabels(doer *user_model.User, issue *issues_model.Issue,
	addedLabels, removedLabels []*issues_model.Label,
) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeLabels(doer, issue, addedLabels, removedLabels)
	}
}

// NotifyCreateRepository notifies create repository to notifiers
func NotifyCreateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRepository(doer, u, repo)
	}
}

// NotifyMigrateRepository notifies create repository to notifiers
func NotifyMigrateRepository(doer, u *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyMigrateRepository(doer, u, repo)
	}
}

// NotifyTransferRepository notifies create repository to notifiers
func NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, newOwnerName string) {
	for _, notifier := range notifiers {
		notifier.NotifyTransferRepository(doer, repo, newOwnerName)
	}
}

// NotifyDeleteRepository notifies delete repository to notifiers
func NotifyDeleteRepository(doer *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRepository(doer, repo)
	}
}

// NotifyForkRepository notifies fork repository to notifiers
func NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyForkRepository(doer, oldRepo, repo)
	}
}

// NotifyRenameRepository notifies repository renamed
func NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldName string) {
	for _, notifier := range notifiers {
		notifier.NotifyRenameRepository(doer, repo, oldName)
	}
}

// NotifyPushCommits notifies commits pushed to notifiers
func NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifyPushCommits(pusher, repo, opts, commits)
	}
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func NotifyCreateRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRef(pusher, repo, refType, refFullName, refID)
	}
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func NotifyDeleteRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRef(pusher, repo, refType, refFullName)
	}
}

// NotifySyncPushCommits notifies commits pushed to notifiers
func NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifySyncPushCommits(pusher, repo, opts, commits)
	}
}

// NotifySyncCreateRef notifies branch or tag creation to notifiers
func NotifySyncCreateRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string) {
	for _, notifier := range notifiers {
		notifier.NotifySyncCreateRef(pusher, repo, refType, refFullName, refID)
	}
}

// NotifySyncDeleteRef notifies branch or tag deletion to notifiers
func NotifySyncDeleteRef(pusher *user_model.User, repo *repo_model.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifySyncDeleteRef(pusher, repo, refType, refFullName)
	}
}

// NotifyRepoPendingTransfer notifies creation of pending transfer to notifiers
func NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *repo_model.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyRepoPendingTransfer(doer, newOwner, repo)
	}
}

// NotifyPackageCreate notifies creation of a package to notifiers
func NotifyPackageCreate(doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.NotifyPackageCreate(doer, pd)
	}
}

// NotifyPackageDelete notifies deletion of a package to notifiers
func NotifyPackageDelete(doer *user_model.User, pd *packages_model.PackageDescriptor) {
	for _, notifier := range notifiers {
		notifier.NotifyPackageDelete(doer, pd)
	}
}
