// Copyright 2018 The Gitea Authors. All rights reserved.
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

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()
	NotifyAdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	NotifyCreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	NotifyMigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	NotifyDeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository)
	NotifyForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository)
	NotifyRenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string)
	NotifyTransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string)
	NotifyNewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User)
	NotifyIssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool)
	NotifyDeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue)
	NotifyIssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64)
	NotifyIssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment)
	NotifyPullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment)
	NotifyIssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string)
	NotifyIssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue)
	NotifyIssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string)
	NotifyIssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldRef string)
	NotifyIssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
		addedLabels, removedLabels []*issues_model.Label)
	NotifyNewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User)
	NotifyMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	NotifyAutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	NotifyPullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	NotifyPullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyPullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyPullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string)
	NotifyPullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment)
	NotifyPullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment)
	NotifyCreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
		issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User)
	NotifyUpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string)
	NotifyDeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment)
	NotifyNewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string)
	NotifyEditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string)
	NotifyDeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string)
	NotifyNewRelease(ctx context.Context, rel *repo_model.Release)
	NotifyUpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release)
	NotifyDeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release)
	NotifyPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifyCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string)
	NotifyDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName)
	NotifySyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifySyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string)
	NotifySyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName)
	NotifyRepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository)
	NotifyPackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor)
	NotifyPackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor)
}
