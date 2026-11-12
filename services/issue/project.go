// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"
)

func AssignOrRemoveProjects(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, newProjectIDs []int64) error {
	added, removed, err := issues_model.IssueAssignOrRemoveProject(ctx, issue, doer, newProjectIDs)
	if err != nil {
		return err
	}
	notify_service.IssueChangeProjects(ctx, doer, issue, added, removed)
	return nil
}
