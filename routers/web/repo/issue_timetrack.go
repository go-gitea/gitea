// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// AddTimeManually tracks time manually
func AddTimeManually(c *context.Context) {
	form := web.GetForm(c).(*forms.AddTimeManuallyForm)
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.Doer) {
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

	if _, err := models.AddTime(c.Doer, issue, int64(total.Seconds()), time.Now()); err != nil {
		c.ServerError("AddTime", err)
		return
	}

	c.Redirect(url, http.StatusSeeOther)
}

// DeleteTime deletes tracked time
func DeleteTime(c *context.Context) {
	issue := GetActionIssue(c)
	if c.Written() {
		return
	}
	if !c.Repo.CanUseTimetracker(issue, c.Doer) {
		c.NotFound("CanUseTimetracker", nil)
		return
	}

	t, err := models.GetTrackedTimeByID(c.ParamsInt64(":timeid"))
	if err != nil {
		if models.IsErrNotExist(err) {
			c.NotFound("time not found", err)
			return
		}
		c.Error(http.StatusInternalServerError, "GetTrackedTimeByID", err.Error())
		return
	}

	// only OP or admin may delete
	if !c.IsSigned || (!c.IsUserSiteAdmin() && c.Doer.ID != t.UserID) {
		c.Error(http.StatusForbidden, "not allowed")
		return
	}

	if err = models.DeleteTime(t); err != nil {
		c.ServerError("DeleteTime", err)
		return
	}

	c.Flash.Success(c.Tr("repo.issues.del_time_history", util.SecToTime(t.Time)))
	c.Redirect(issue.HTMLURL())
}
