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
	"gitea.dev/modules/log"
	"gitea.dev/modules/repository"
	notify_service "gitea.dev/services/notify"
)

type notifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &notifier{}

func checkQueueOfPull(ctx context.Context, pr *issues_model.PullRequest) {
	entry, err := pull_model.GetMergeQueueEntryByPullID(ctx, pr.ID)
	if err != nil {
		log.Error("GetMergeQueueEntryByPullID: %v", err)
	} else if entry != nil {
		addToQueue(entry.RepoID, entry.BaseBranch)
	}
}

func (n *notifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
	if status.State.IsPending() {
		return // can't change an entry awaiting checks, timeouts are enforced by the cron task
	}
	entries, err := pull_model.GetMergeQueueEntriesByGroupHeadSHA(ctx, repo.ID, commit.Sha1)
	if err != nil {
		log.Error("GetMergeQueueEntriesByGroupHeadSHA: %v", err)
		return
	}
	for _, e := range entries {
		addToQueue(e.RepoID, e.BaseBranch)
	}
}

func (n *notifier) PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, before, after string) {
	checkQueueOfPull(ctx, pr)
}

func (n *notifier) PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, oldBranch, oldSHA string) {
	checkQueueOfPull(ctx, pr)
}

func (n *notifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	if !isClosed || !issue.IsPull {
		return
	}
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest: %v", err)
		return
	}
	checkQueueOfPull(ctx, issue.PullRequest)
}

func (n *notifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	if !opts.RefFullName.IsBranch() {
		return
	}
	branch := opts.RefFullName.BranchName()
	if has, err := pull_model.HasMergeQueueEntries(ctx, repo.ID, branch); err != nil {
		log.Error("HasMergeQueueEntries: %v", err)
	} else if has {
		addToQueue(repo.ID, branch)
	}
}
