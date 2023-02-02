// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// Cron represents a Cron task
type Cron struct {
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Next      time.Time `json:"next"`
	Prev      time.Time `json:"prev"`
	ExecTimes int64     `json:"exec_times"`
}
