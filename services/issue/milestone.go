// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeMilestoneAssign changes assignment of milestone for issue.
func ChangeMilestoneAssign(issue *models.Issue, doer *user_model.User, oldMilestoneID int64) (err error) {
	if err = models.ChangeMilestoneAssign(issue, doer, oldMilestoneID); err != nil {
		return
	}

	notification.NotifyIssueChangeMilestone(doer, issue, oldMilestoneID)

	return nil
}
