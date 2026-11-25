// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mergequeue

import (
	"context"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/repository"
	notify_service "gitea.dev/services/notify"

	"xorm.io/builder"
)

type notifier struct{ notify_service.NullNotifier }

func checkQueueOfPull(ctx context.Context, pullID int64) {
	if err := scheduleQueues(ctx, builder.Eq{"pull_id": pullID}); err != nil {
		log.Error("checkQueueOfPull: %v", err)
	}
}

func (*notifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, _sender *user_model.User, _status *git_model.CommitStatus) {
	if err := scheduleQueues(ctx, builder.Eq{"repo_id": repo.ID, "group_head_sha": commit.Sha1}); err != nil {
		log.Error("CreateCommitStatus: %v", err)
	}
}

func (*notifier) PullRequestSynchronized(ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, _before, _after string) {
	checkQueueOfPull(ctx, pr.ID)
}

func (*notifier) PullRequestChangeTargetBranch(ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, _oldBranch, _oldSHA string) {
	checkQueueOfPull(ctx, pr.ID)
}

func (*notifier) IssueChangeStatus(ctx context.Context, _doer *user_model.User, _commitID string, issue *issues_model.Issue, _actionComment *issues_model.Comment, isClosed bool) {
	if !isClosed || !issue.IsPull {
		return
	}
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest: %v", err)
		return
	}
	checkQueueOfPull(ctx, issue.PullRequest.ID)
}

func (*notifier) PushCommits(ctx context.Context, _pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, _commits *repository.PushCommits) {
	checkQueueOfBranch(ctx, repo, opts.RefFullName)
}

func (*notifier) CreateRef(ctx context.Context, _doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, _refID string) {
	checkQueueOfBranch(ctx, repo, refFullName)
}

func (*notifier) DeleteRef(ctx context.Context, _doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	checkQueueOfBranch(ctx, repo, refFullName)
}

func checkQueueOfBranch(ctx context.Context, repo *repo_model.Repository, refFullName git.RefName) {
	branch := refFullName.BranchName()
	if branch == "" {
		return
	}
	if has, err := pull_model.HasMergeQueueEntries(ctx, repo.ID, branch); err != nil {
		log.Error("HasMergeQueueEntries: %v", err)
	} else if has {
		addToQueue(repo.ID, branch)
	}
}
