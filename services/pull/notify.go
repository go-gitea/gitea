// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

type pullNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &pullNotifier{}

// newNotifier create a new indexerNotifier notifier
func newNotifier() notify_service.Notifier {
	return &pullNotifier{}
}

func (r *pullNotifier) IssueChangeTitle(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldTitle string) {
	var reviewNotifiers []*ReviewRequestNotifier
	if issue.IsPull && issues_model.HasWorkInProgressPrefix(oldTitle) && !issues_model.HasWorkInProgressPrefix(issue.Title) {
		var err error
		reviewNotifiers, err = RequestCodeOwnersReview(ctx, issue.PullRequest)
		if err != nil {
			log.Error("RequestCodeOwnersReview: %v", err)
		}
	}
	ReviewRequestNotify(ctx, issue, issue.Poster, reviewNotifiers)
}
