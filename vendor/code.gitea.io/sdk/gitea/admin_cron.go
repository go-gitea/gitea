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
func (c *Client) ListCronTasks(opt ListCronTaskOptions) ([]*CronTask, error) {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return nil, err
	}
	opt.setDefaults()
	ct := make([]*CronTask, 0, opt.PageSize)
	return ct, c.getParsedResponse("GET", fmt.Sprintf("/admin/cron?%s", opt.getURLQuery().Encode()), jsonHeader, nil, &ct)
}

// RunCronTasks run a cron task
func (c *Client) RunCronTasks(task string) error {
	if err := c.CheckServerVersionConstraint(">=1.13.0"); err != nil {
		return err
	}
	_, err := c.getResponse("POST", fmt.Sprintf("/admin/cron/%s", task), jsonHeader, nil)
	return err
}
