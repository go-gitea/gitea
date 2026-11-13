// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
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

func (n *automergeNotifier) PullRequestChangeTargetBranch(ctx context.Context, _doer *user_model.User, pr *issues_model.PullRequest, _oldBranch, _oldSHA string) {
	// a writer's retarget keeps the schedule, and the new base may have made the pull request mergeable
	automergequeue.StartAutoMergeCheckByPullHead(ctx, pr)
}

func disableIssueAutoMerge(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reason string) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		log.Error("LoadPullRequest[%d]: %v", issue.ID, err)
	} else if err := pull_service.RemoveScheduledAutoMerge(ctx, doer, issue.PullRequest, reason); err != nil {
		log.Error("RemoveScheduledAutoMerge[%d]: %v", issue.ID, err)
	}
}

func (n *automergeNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, _commitID string, issue *issues_model.Issue, _actionComment *issues_model.Comment, isClosed bool) {
	if isClosed && issue.IsPull {
		disableIssueAutoMerge(ctx, doer, issue, "closed")
	}
}

func (n *automergeNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	if issue.IsPull && !issues_model.HasWorkInProgressPrefix(oldTitle) && issues_model.HasWorkInProgressPrefix(issue.Title) {
		disableIssueAutoMerge(ctx, doer, issue, "work_in_progress")
	}
}
