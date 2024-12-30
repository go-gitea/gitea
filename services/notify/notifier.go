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
	"code.gitea.io/gitea/modules/repository"
)

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()

	AdoptRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	MigrateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository)
	DeleteRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository)
	ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository)
	RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string)
	TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string)
	RepoPendingTransfer(ctx context.Context, doer, newOwner *user_model.User, repo *repo_model.Repository)

	NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User)
	IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool)
	DeleteIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue)
	IssueChangeMilestone(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldMilestoneID int64)
	IssueChangeAssignee(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, assignee *user_model.User, removed bool, comment *issues_model.Comment)
	PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isRequest bool, comment *issues_model.Comment)
	IssueChangeContent(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldContent string)
	IssueClearLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue)
	IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string)
	IssueChangeRef(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldRef string)
	IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue,
		addedLabels, removedLabels []*issues_model.Label)

	NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User)
	MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest)
	PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User)
	PullRequestCodeComment(ctx context.Context, pr *issues_model.PullRequest, comment *issues_model.Comment, mentions []*user_model.User)
	PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch string)
	PullRequestPushCommits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, comment *issues_model.Comment)
	PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment)

	CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
		issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User)
	UpdateComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment, oldContent string)
	DeleteComment(ctx context.Context, doer *user_model.User, c *issues_model.Comment)

	NewWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string)
	EditWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page, comment string)
	DeleteWikiPage(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, page string)

	NewRelease(ctx context.Context, rel *repo_model.Release)
	UpdateRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release)
	DeleteRelease(ctx context.Context, doer *user_model.User, rel *repo_model.Release)

	PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	CreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string)
	DeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName)
	SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	SyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string)
	SyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName)

	PackageCreate(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor)
	PackageDelete(ctx context.Context, doer *user_model.User, pd *packages_model.PackageDescriptor)

	ChangeDefaultBranch(ctx context.Context, repo *repo_model.Repository)

	CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus)
}
