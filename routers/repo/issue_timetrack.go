// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
)

// AddTimeManually tracks time manually
func AddTimeManually(c *context.Context, form auth.AddTimeManuallyForm) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.User) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}
	url := issue.HTMLURL()

	if c.HasError() {
		c.Flash.Error(c.GetErrMsg())
		c.Redirect(url)
		return
	}

	total := time.Duration(form.Hours)*time.Hour + time.Duration(form.Minutes)*time.Minute

	if total <= 0 {
		c.Flash.Error(c.Tr("repo.issues.add_time_sum_to_small"))
		c.Redirect(url, http.StatusSeeOther)
		return
	}

	if _, err := models.AddTime(c.User, issue, int64(total.Seconds()), time.Now()); err != nil {
		c.ServerError("AddTime", err)
		return
	}

	c.Redirect(url, http.StatusSeeOther)
}
