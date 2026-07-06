// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(NewNotifier())
}

type aireviewNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &aireviewNotifier{}

// NewNotifier creates a new AI review notifier.
func NewNotifier() notify_service.Notifier {
	return &aireviewNotifier{}
}

func (n *aireviewNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, _ []*user_model.User) {
	if !setting.AIRreview.Enabled || !setting.AIRreview.TriggerOnOpen {
		return
	}
	pushTask(pr.ID, "opened")
}

func (n *aireviewNotifier) PullRequestSynchronized(ctx context.Context, _ *user_model.User, pr *issues_model.PullRequest, _, _ string) {
	if !setting.AIRreview.Enabled || !setting.AIRreview.TriggerOnUpdate {
		return
	}
	pushTask(pr.ID, "synchronized")
}

func (n *aireviewNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, _ []*user_model.User) {
	if err := HandlePRComment(ctx, doer, repo, issue, comment); err != nil {
		log.Error("aireview: HandlePRComment failed: %v", err)
	}
}

func pushTask(prID int64, event string) {
	if reviewQueue == nil {
		log.Warn("aireview: queue not initialized, skipping PR %d review", prID)
		return
	}
	if err := reviewQueue.Push(AIRreviewTask{PRID: prID, Event: event}); err != nil {
		log.Error("aireview: failed to enqueue PR %d: %v", prID, err)
	}
}
