// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// ChangeStatus changes issue status to open or closed.
func ChangeStatus(issue *models.Issue, doer *models.User, isClosed bool) (err error) {
	comment, err := issue.ChangeStatus(doer, isClosed)
	if err != nil {
		return
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, isClosed)
	return nil
}
