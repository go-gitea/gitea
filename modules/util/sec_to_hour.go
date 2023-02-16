// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
)

// SecToHour works similarly to SecToTime (i.e. converts an amount of seconds
// to a human-readable string), but works with units that work in a timesheet,
// namely: only hours and minutes.
//
// If somebody worked 8 hours on 4 workdays on an issue (4 days * 8 hours), we
// need to see "32 hours", not "1 day 8 hours". When dealing with worktime, no
// project manager calculates like that.
//
// For example:
// 66      -> 1 minute
// 52410   -> 14 hours 33 minutes
// 563418  -> 156 hours 30 minutes (NOT "6 days 12 hours")
func SecToHour(duration int64) string {
	formattedTime := ""
	hours := (duration / 3600)
	minutes := (duration / 60) % 60

	// Show hours if any
	if hours > 0 {
		formattedTime = formatTime(hours, "hour", formattedTime)
	}
	// Show minutes always
	formattedTime = formatTime(minutes, "minute", formattedTime)

	// The formatTime() function always appends a space at the end. This will be trimmed
	return strings.TrimRight(formattedTime, " ")
}
