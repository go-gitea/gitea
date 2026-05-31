// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	notify_service "gitea.dev/services/notify"
)

// ChangeDeadline changes an issue deadline and notifies issue listeners.
func ChangeDeadline(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, deadlineUnix timeutil.TimeStamp) error {
	oldDeadlineUnix := issue.DeadlineUnix
	if oldDeadlineUnix == deadlineUnix {
		return nil
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return err
	}

	if err := issues_model.UpdateIssueDeadline(ctx, issue, deadlineUnix, doer); err != nil {
		return err
	}

	issue.DeadlineUnix = deadlineUnix
	notify_service.IssueChangeDeadline(ctx, doer, issue, oldDeadlineUnix)

	return nil
}
