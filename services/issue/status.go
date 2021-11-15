// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeStatus changes issue status to open or closed.
func ChangeStatus(issue *models.Issue, doer *models.User, closed bool) error {
	stopTimerIfAvailable := func(doer *models.User, issue *models.Issue) error {
		if models.StopwatchExists(doer.ID, issue.ID) {
			if err := models.CreateOrStopIssueStopwatch(doer, issue); err != nil {
				return err
			}
		}

		return nil
	}

	comment, err := issue.ChangeStatus(doer, closed)
	if err != nil {
		// Don't return an error when dependencies are open as this would let the push fail
		if models.IsErrDependenciesLeft(err) {
			return stopTimerIfAvailable(doer, issue)
		}
		return err
	}

	if err := stopTimerIfAvailable(doer, issue); err != nil {
		return err
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, closed)

	return nil
}
