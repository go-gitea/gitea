// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// Cron represents a Cron task
type Cron struct {
	// The name of the cron task
	Name string `json:"name"`
	// The cron schedule expression (e.g., "0 0 * * *")
	Schedule string `json:"schedule"`
	// The next scheduled execution time
	Next time.Time `json:"next"`
	// The previous execution time
	Prev time.Time `json:"prev"`
	// The total number of times this cron task has been executed
	ExecTimes int64 `json:"exec_times"`
}
