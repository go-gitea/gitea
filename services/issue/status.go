// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
func ChangeStatus(issue *issues_model.Issue, doer *user_model.User, closed bool) error {
	return changeStatusCtx(db.DefaultContext, issue, doer, closed)
}

// changeStatusCtx changes issue status to open or closed.
// TODO: if context is not db.DefaultContext we get a deadlock!!!
func changeStatusCtx(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, closed bool) error {
	comment, err := issues_model.ChangeIssueStatus(ctx, issue, doer, closed)
	if err != nil {
		if issues_model.IsErrDependenciesLeft(err) && closed {
			if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
				log.Error("Unable to stop stopwatch for issue[%d]#%d: %v", issue.ID, issue.Index, err)
			}
		}
		return err
	}

	if closed {
		if err := issues_model.FinishIssueStopwatchIfPossible(ctx, doer, issue); err != nil {
			return err
		}
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, closed)

	return nil
}
