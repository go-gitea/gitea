// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"time"
	"strconv"
	"net/http"

	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// AddTimeManually tracks time manually
func AddTimeManually(c *context.Context, form auth.AddTimeManuallyForm) {
	issueIndex := c.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(c.Repo.Repository.ID, issueIndex)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}
	url := issue.HTMLURL()

	if c.HasError() {
		c.Flash.Error(c.GetErrMsg())
		c.Redirect(url)
		return
	}

	h, err := parseTimeTrackingWithDuration(form.Hours, "h")
	if err != nil {
		c.Handle(http.StatusInternalServerError, "parseTimeTrackingWithDuration", err)
		return
	}

	m, err := parseTimeTrackingWithDuration(form.Minutes, "m")
	if err != nil {
		c.Handle(http.StatusInternalServerError, "parseTimeTrackingWithDuration", err)
		return
	}



	total := h + m

	if total <= 0 {
		c.Flash.Error(c.Tr("repo.issues.add_time_sum_to_small"))
		c.Redirect(url, http.StatusSeeOther)
		return
	}

	if err := models.AddTime(c.User.ID, issue.ID, int64(total.Seconds())); err != nil {
		c.Handle(http.StatusInternalServerError, "AddTime", err)
		return
	}

	c.Redirect(url, http.StatusSeeOther)
}

func parseTimeTrackingWithDuration(value int, space string) (time.Duration, error) {
	return time.ParseDuration(strconv.Itoa(value) + space)
}
