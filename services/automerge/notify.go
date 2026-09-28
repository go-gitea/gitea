// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	pull_model "gitea.dev/models/pull"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/repository"
	"gitea.dev/services/automergequeue"
	notify_service "gitea.dev/services/notify"
	pull_service "gitea.dev/services/pull"
)

type automergeNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &automergeNotifier{}

// NewNotifier create a new automergeNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &automergeNotifier{}
}

func (n *automergeNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	// as a missing / blocking reviews could have blocked a pending automerge let's recheck
	if review.Type == issues_model.ReviewTypeApprove {
		automergequeue.StartAutoMergeCheckByPullHead(ctx, pr)
	}
}

func (n *automergeNotifier) PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	if err := review.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := review.Issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest: %v", err)
		return
	}
	// as reviews could have blocked a pending automerge let's recheck
	automergequeue.StartAutoMergeCheckByPullHead(ctx, review.Issue.PullRequest)
}

func (n *automergeNotifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
	if !status.State.IsSuccess() {
		return
	}

	pulls, err := pull_service.GetMergeablePullRequestsByHeadCommitID(ctx, repo, commit.Sha1)
	if err != nil {
		log.Error("GetMergeablePullRequestsByHeadCommitID: %v", err)
		return
	}
	for _, pr := range pulls {
		automergequeue.StartAutoMergeCheckByPullHead(ctx, pr)
	}
}

func disableIssueAutoMerge(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reason string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest[%d]: %v", issue.ID, err)
	} else if err := RemoveScheduledAutoMerge(ctx, doer, issue.PullRequest, reason); err != nil {
		log.Error("RemoveScheduledAutoMerge[%d]: %v", issue.ID, err)
	}
}

// disableAutoMergeIfNotWriter disables auto merge on changes by non-writers, like GitHub
func disableAutoMergeIfNotWriter(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, reason string) error {
	if exist, _, err := pull_model.GetScheduledMergeByPullID(ctx, pr.ID); err != nil || !exist {
		return err
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return err
	}
	if perm, err := access_model.GetDoerRepoPermission(ctx, pr.BaseRepo, doer); err != nil || perm.CanWrite(unit.TypeCode) {
		return err
	}
	return RemoveScheduledAutoMerge(ctx, doer, pr, reason)
}

func (n *automergeNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, _ string, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	if isClosed && issue.IsPull {
		disableIssueAutoMerge(ctx, doer, issue, "closed")
	}
}

// IssueChangeTitle disables auto merge when a WIP prefix is added, like GitHub's convert to draft
func (n *automergeNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if issue.IsPull && !issues_model.HasWorkInProgressPrefix(oldTitle) && issues_model.HasWorkInProgressPrefix(issue.Title) {
		disableIssueAutoMerge(ctx, doer, issue, "work_in_progress")
	}
}

func (n *automergeNotifier) PullRequestSynchronized(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, _, _ string) {
	// pushing to a same-repo head branch already requires write access, and Actions or deploy key pushers have no user permission
	if pr.Flow == issues_model.PullRequestFlowGithub && pr.HeadRepoID == pr.BaseRepoID {
		return
	}
	if err := disableAutoMergeIfNotWriter(ctx, doer, pr, "pushed_by_non_writer"); err != nil {
		log.Error("disableAutoMergeIfNotWriter[%d]: %v", pr.ID, err)
	}
}

func (n *automergeNotifier) PullRequestChangeTargetBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, _ string) {
	if err := disableAutoMergeIfNotWriter(ctx, doer, pr, "base_changed"); err != nil {
		log.Error("disableAutoMergeIfNotWriter[%d]: %v", pr.ID, err)
	}
}
