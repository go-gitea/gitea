// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package automerge

import (
	"context"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	notify_service "code.gitea.io/gitea/services/notify"
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
		if err := StartPRCheckAndAutoMergeBySHA(ctx, review.CommitID, pr.BaseRepo); err != nil {
			log.Error("StartPullRequestAutoMergeCheckBySHA: %v", err)
		}
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
	StartPRCheckAndAutoMerge(ctx, review.Issue.PullRequest)
}

func (n *automergeNotifier) CreateCommitStatus(ctx context.Context, repo *repo_model.Repository, commit *repository.PushCommit, sender *user_model.User, status *git_model.CommitStatus) {
	if status.State.IsSuccess() {
		if err := StartPRCheckAndAutoMergeBySHA(ctx, commit.Sha1, repo); err != nil {
			log.Error("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %w", repo.ID, sender.ID, commit.Sha1, err)
		}
	}
}
