// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	issues_model "code.gitea.io/gitea/models/issues"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/repository"
)

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()
	NotifyCreateRepository(doer, u *user_model.User, repo *repo_model.Repository)
	NotifyMigrateRepository(doer, u *user_model.User, repo *repo_model.Repository)
	NotifyDeleteRepository(doer *user_model.User, repo *repo_model.Repository)
	NotifyForkRepository(doer *user_model.User, oldRepo, repo *repo_model.Repository)
	NotifyRenameRepository(doer *user_model.User, repo *repo_model.Repository, oldRepoName string)
	NotifyTransferRepository(doer *user_model.User, repo *repo_model.Repository, oldOwnerName string)
	NotifyNewIssue(issue *issues_model.Issue, mentions []*user_model.User)
	NotifyIssueChangeStatus(*user_model.User, *issues_model.Issue, *issues_model.Comment, bool)
	NotifyDeleteIssue(*user_model.User, *issues_model.Issue)
	NotifyIssueChangeMilestone(doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64)
	NotifyIssueChangeAssignee(doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment)
	NotifyPullReviewRequest(doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment)
	NotifyIssueChangeContent(doer *user_model.User, issue *issues_model.Issue, oldContent string)
	NotifyIssueClearLabels(doer *user_model.User, issue *issues_model.Issue)
	NotifyIssueChangeTitle(doer *user_model.User, issue *issues_model.Issue, oldTitle string)
	NotifyIssueChangeRef(doer *user_model.User, issue *issues_model.Issue, oldRef string)
	NotifyIssueChangeLabels(doer *user_model.User, issue *issues_model.Issue,
		addedLabels, removedLabels []*issues_model.Label)
	NotifyNewPullRequest(pr *issues_model.PullRequest, mentions []*user_model.User)
	NotifyMergePullRequest(*issues_model.PullRequest, *user_model.User)
	NotifyPullRequestSynchronized(doer *user_model.User, pr *issues_model.PullRequest)
	NotifyPullRequestReview(pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyPullRequestCodeComment(pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *issues_model.PullRequest, oldBranch string)
	NotifyPullRequestPushCommits(doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment)
	NotifyPullRevieweDismiss(doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment)
	NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
		issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyUpdateComment(*user_model.User, *issues_model.Comment, string)
	NotifyDeleteComment(*user_model.User, *issues_model.Comment)
	NotifyNewRelease(rel *models.Release)
	NotifyUpdateRelease(doer *user_model.User, rel *models.Release)
	NotifyDeleteRelease(doer *user_model.User, rel *models.Release)
	NotifyPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifyCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string)
	NotifyDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string)
	NotifySyncPushCommits(pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifySyncCreateRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName, refID string)
	NotifySyncDeleteRef(doer *user_model.User, repo *repo_model.Repository, refType, refFullName string)
	NotifyRepoPendingTransfer(doer, newOwner *user_model.User, repo *repo_model.Repository)
	NotifyPackageCreate(doer *user_model.User, pd *packages_model.PackageDescriptor)
	NotifyPackageDelete(doer *user_model.User, pd *packages_model.PackageDescriptor)
}
