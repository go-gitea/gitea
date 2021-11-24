// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeStatus changes issue status to open or closed.
func ChangeStatus(issue *models.Issue, doer *user_model.User, closed bool) error {
	comment, err := issue.ChangeStatus(doer, closed)
	if err != nil {
		// Don't return an error when dependencies are open as this would let the push fail
		if models.IsErrDependenciesLeft(err) {
			if closed {
				return models.FinishIssueStopwatchIfPossible(db.DefaultContext, doer, issue)
			}
		}
		return err
	}

	if closed {
		if err := models.FinishIssueStopwatchIfPossible(db.DefaultContext, doer, issue); err != nil {
			return err
		}
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, closed)

	return nil
}
