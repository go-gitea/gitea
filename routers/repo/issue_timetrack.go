// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"net/http"
	"strconv"
)

// AddTimeManual tracks time manually
func AddTimeManual(c *context.Context) {
	hours, err := strconv.ParseInt(c.Req.PostForm.Get("hours"), 10, 64)
	if err != nil {
		if c.Req.PostForm.Get("hours") != "" {
			c.Handle(http.StatusInternalServerError, "hours is not numeric", err)
			return
		}
	}
	minutes, err := strconv.ParseInt(c.Req.PostForm.Get("minutes"), 10, 64)
	if err != nil {
		if c.Req.PostForm.Get("minutes") != "" {
			c.Handle(http.StatusInternalServerError, "minutes is not numeric", err)
			return
		}
	}
	seconds, err := strconv.ParseInt(c.Req.PostForm.Get("seconds"), 10, 64)
	if err != nil {
		if c.Req.PostForm.Get("seconds") != "" {
			c.Handle(http.StatusInternalServerError, "seconds is not numeric", err)
			return 
		}
	}

	totalInSeconds := seconds + minutes*60 + hours*60*60

	if totalInSeconds <= 0 {
		c.Handle(http.StatusInternalServerError, "sum of seconds <= 0", nil)
		return
	}

	issueIndex := c.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(c.Repo.Repository.ID, issueIndex)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	if err := models.AddTime(c.User.ID, issue.ID, totalInSeconds); err != nil {
		c.Handle(http.StatusInternalServerError, "AddTime", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}
