// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

type webhookNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &webhookNotifier{}

// NewNotifier create a new webhooksNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &webhookNotifier{}
}

func (n *webhookNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User) {
	// TODO
}
