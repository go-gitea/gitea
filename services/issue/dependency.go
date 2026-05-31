// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"
)

// CreateIssueDependency creates a dependency and notifies subscribers.
func CreateIssueDependency(ctx context.Context, doer *user_model.User, issue, dependency *issues_model.Issue) error {
	if err := issues_model.CreateIssueDependency(ctx, doer, issue, dependency); err != nil {
		return err
	}

	notify_service.IssueChangeDependency(ctx, doer, issue, dependency, true)
	return nil
}

// RemoveIssueDependency removes a dependency and notifies subscribers.
func RemoveIssueDependency(ctx context.Context, doer *user_model.User, issue, dependency *issues_model.Issue, depType issues_model.DependencyType) error {
	if err := issues_model.RemoveIssueDependency(ctx, doer, issue, dependency, depType); err != nil {
		return err
	}

	notify_service.IssueChangeDependency(ctx, doer, issue, dependency, false)
	return nil
}
