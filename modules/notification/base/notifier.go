// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
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
	NotifyNewIssue(issue *models.Issue, mentions []*user_model.User)
	NotifyIssueChangeStatus(*user_model.User, *models.Issue, *models.Comment, bool)
	NotifyDeleteIssue(*user_model.User, *models.Issue)
	NotifyIssueChangeMilestone(doer *user_model.User, issue *models.Issue, oldMilestoneID int64)
	NotifyIssueChangeAssignee(doer *user_model.User, issue *models.Issue, assignee *user_model.User, removed bool, comment *models.Comment)
	NotifyPullReviewRequest(doer *user_model.User, issue *models.Issue, reviewer *user_model.User, isRequest bool, comment *models.Comment)
	NotifyIssueChangeContent(doer *user_model.User, issue *models.Issue, oldContent string)
	NotifyIssueClearLabels(doer *user_model.User, issue *models.Issue)
	NotifyIssueChangeTitle(doer *user_model.User, issue *models.Issue, oldTitle string)
	NotifyIssueChangeRef(doer *user_model.User, issue *models.Issue, oldRef string)
	NotifyIssueChangeLabels(doer *user_model.User, issue *models.Issue,
		addedLabels, removedLabels []*models.Label)
	NotifyNewPullRequest(pr *models.PullRequest, mentions []*user_model.User)
	NotifyMergePullRequest(*models.PullRequest, *user_model.User)
	NotifyPullRequestSynchronized(doer *user_model.User, pr *models.PullRequest)
	NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*user_model.User)
	NotifyPullRequestCodeComment(pr *models.PullRequest, comment *models.Comment, mentions []*user_model.User)
	NotifyPullRequestChangeTargetBranch(doer *user_model.User, pr *models.PullRequest, oldBranch string)
	NotifyPullRequestPushCommits(doer *user_model.User, pr *models.PullRequest, comment *models.Comment)
	NotifyPullRevieweDismiss(doer *user_model.User, review *models.Review, comment *models.Comment)
	NotifyCreateIssueComment(doer *user_model.User, repo *repo_model.Repository,
		issue *models.Issue, comment *models.Comment, mentions []*user_model.User)
	NotifyUpdateComment(*user_model.User, *models.Comment, string)
	NotifyDeleteComment(*user_model.User, *models.Comment)
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
}
