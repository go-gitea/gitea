// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeStatus changes issue status to open or closed.
func ChangeStatus(issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	return changeStatusCtx(db.DefaultContext, issue, doer, commitID)
}

// changeStatusCtx changes issue status to open or closed.
// TODO: if context is not db.DefaultContext we get a deadlock!!!
func changeStatusCtx(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ChangeIssueStatus(ctx, issue, doer)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) && issue.IsClosed {
			if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
				log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
			}
		}
		return err
	}

	if issue.IsClosed {
		if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
			return err
		}
	}

	// TBD: whether to notify if only closed_status is changed.
	notification.NotifyIssueChangeStatus(ctx, doer, commitID, issue, comment, issue.IsClosed)

	return nil
}
