// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// ClearLabels clears all of an issue's labels
func ClearLabels(issue *models.Issue, doer *models.User) (err error) {
	if err = issue.ClearLabels(doer); err != nil {
		return
	}

	notification.NotifyIssueClearLabels(doer, issue)

	return nil
}
