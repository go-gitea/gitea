// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"strings"
)

// SecToHours converts an amount of seconds to a human-readable hours string.
// This is stable for planning and managing timesheets.
// Here it only supports hours and minutes, because a work day could contain 6 or 7 or 8 hours.
// If the duration is less than 1 minute, it will be shown as seconds.
func SecToHours(durationVal any) string {
	seconds, _ := ToInt64(durationVal)
	hours := seconds / 3600
	minutes := (seconds / 60) % 60

	formattedTime := ""
	formattedTime = formatTime(hours, "hour", formattedTime)
	formattedTime = formatTime(minutes, "minute", formattedTime)

	// The formatTime() function always appends a space at the end. This will be trimmed
	if formattedTime == "" && seconds > 0 {
		formattedTime = formatTime(seconds, "second", "")
	}
	return strings.TrimRight(formattedTime, " ")
}

// formatTime appends the given value to the existing forammattedTime. E.g:
// formattedTime = "1 year"
// input: value = 3, name = "month"
// output will be "1 year 3 months "
func formatTime(value int64, name, formattedTime string) string {
	if value == 1 {
		formattedTime = fmt.Sprintf("%s1 %s ", formattedTime, name)
	} else if value > 1 {
		formattedTime = fmt.Sprintf("%s%d %ss ", formattedTime, value, name)
	}
	return formattedTime
}
