// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"net/http"
	"time"
	"fmt"
)

// AddTimeManual tracks time manually
func AddTimeManual(c *context.Context) {

	h, err := parseTimeTrackingWithDuration(c.Req.PostForm.Get("hours"), "h")
	if err != nil {
		fmt.Println("hours is not numeric", err)
		c.Handle(http.StatusBadRequest, "hours is not numeric", err)
		return
	}

	m, err := parseTimeTrackingWithDuration(c.Req.PostForm.Get("minutes"), "m")
	if err != nil {
		fmt.Println("minutes is not numeric", err)
		c.Handle(http.StatusBadRequest, "minutes is not numeric", err)
		return
	}

	s, err := parseTimeTrackingWithDuration(c.Req.PostForm.Get("seconds"), "s")
	if err != nil {
		fmt.Println("seconds is not numeric", err)
		c.Handle(http.StatusBadRequest, "seconds is not numeric", err)
		return
	}

	totalInSeconds := h.Seconds() + m.Seconds() + s.Seconds()

	if totalInSeconds <= 0 {
		c.Handle(http.StatusBadRequest, "sum of seconds <= 0", nil)
		return
	}

	issueIndex := c.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(c.Repo.Repository.ID, issueIndex)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	if err := models.AddTime(c.User.ID, issue.ID, int64(totalInSeconds)); err != nil {
		c.Handle(http.StatusInternalServerError, "AddTime", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}


func parseTimeTrackingWithDuration(value, space string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value + space)
}

