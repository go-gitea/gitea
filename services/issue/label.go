// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// ClearLabels clear an issue's all labels
func ClearLabels(issue *models.Issue, doer *models.User) (err error) {
	if err = issue.ClearLabels(doer); err != nil {
		return
	}

	if err = issue.LoadPoster(); err != nil {
		return fmt.Errorf("loadPoster: %v", err)
	}

	notification.NotifyIssueClearLabels(doer, issue)

	return nil
}
