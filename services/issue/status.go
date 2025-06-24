// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	notify_service "code.gitea.io/gitea/services/notify"
)

// CloseIssue close an issue.
func CloseIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	comment, err := issues_model.CloseIssue(dbCtx, issue, doer)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) {
			if err := issues_model.FinishIssueStopwatchIfPossible(dbCtx, doer, issue); err != nil {
				log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
			}
		}
		return err
	}

	if err := issues_model.FinishIssueStopwatchIfPossible(dbCtx, doer, issue); err != nil {
		return err
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	committer.Close()

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, true)

	return nil
}

// ReopenIssue reopen an issue.
// FIXME: If some issues dependent this one are closed, should we also reopen them?
func ReopenIssue(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, commitID string) error {
	comment, err := issues_model.ReopenIssue(ctx, issue, doer)
	if err != nil {
		return err
	}

	notify_service.IssueChangeStatus(ctx, doer, commitID, issue, comment, false)

	return nil
}
