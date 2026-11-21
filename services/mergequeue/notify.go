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
)

type notifier struct{ notify_service.NullNotifier }

func checkQueueOfPull(ctx context.Context, pr *issues_model.PullRequest) {
	entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
	if err != nil {
		log.Error("GetMergeQueueEntryByPullID: %v", err)
	} else if entry != nil {
		addToQueue(entry.RepoID, entry.BaseBranch)
	}
}

func (*notifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, _sender *user_model.User, _status *git_model.CommitStatus) {
	if err := scheduleQueues(pull_model.GetMergeQueueEntriesByGroupHeadSHA(ctx, repo.ID, commit.Sha1)); err != nil {
		log.Error("GetMergeQueueEntriesByGroupHeadSHA: %v", err)
	}
}

func (*notifier) PullRequestSynchronized(ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, _before, _after string) {
	checkQueueOfPull(ctx, pr)
}

func (*notifier) PullRequestChangeTargetBranch(ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, _oldBranch, _oldSHA string) {
	checkQueueOfPull(ctx, pr)
}

func (*notifier) IssueChangeStatus(ctx context.Context, _doer *user_model.User, _commitID string, issue *issues_model.Issue, _actionComment *issues_model.Comment, isClosed bool) {
	if !isClosed || !issue.IsPull {
		return
	}
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest: %v", err)
		return
	}
	checkQueueOfPull(ctx, issue.PullRequest)
}

func (*notifier) PushCommits(ctx context.Context, _pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, _commits *repository.PushCommits) {
	checkQueueOfBranch(ctx, repo, opts.RefFullName)
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
