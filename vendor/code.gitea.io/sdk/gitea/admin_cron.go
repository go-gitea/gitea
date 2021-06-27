// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"time"
)

// CronTask represents a Cron task
type CronTask struct {
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Next      time.Time `json:"next"`
	Prev      time.Time `json:"prev"`
	ExecTimes int64     `json:"exec_times"`
}

// ListCronTaskOptions list options for ListCronTasks
type ListCronTaskOptions struct {
	ListOptions
}

// ListCronTasks list available cron tasks
func (c *Client) ListCronTasks(opt ListCronTaskOptions) ([]*CronTask, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	ct := make([]*CronTask, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/admin/cron?%s", opt.getURLQuery().Encode()), jsonHeader, nil, &ct)
	return ct, resp, err
}

// RunCronTasks run a cron task
func (c *Client) RunCronTasks(task string) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&task); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("POST", fmt.Sprintf("/admin/cron/%s", task), jsonHeader, nil)
	return resp, err
}
