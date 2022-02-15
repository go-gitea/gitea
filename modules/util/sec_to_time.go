// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "fmt"

// SecToTime converts an amount of seconds to a human-readable string (example: 66s -> 1min 6s)
func SecToTime(duration int64) string {
	seconds := duration % 60
	minutes := (duration / (60)) % 60
	hours := duration / (60 * 60) % 24
	days := duration / (60 * 60) / 24

	var formattedTime string

	if days > 0 {
		formattedTime = fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if formattedTime == "" {
			formattedTime = fmt.Sprintf("%dh", hours)
		} else {
			formattedTime = fmt.Sprintf("%s %dh", formattedTime, hours)
		}
	}
	if minutes > 0 {
		if formattedTime == "" {
			formattedTime = fmt.Sprintf("%dm", minutes)
		} else {
			formattedTime = fmt.Sprintf("%s %dm", formattedTime, minutes)
		}
	}
	if seconds > 0 {
		if formattedTime == "" {
			formattedTime = fmt.Sprintf("%ds", seconds)
		} else {
			formattedTime = fmt.Sprintf("%s %ds", formattedTime, seconds)
		}
	}

	return formattedTime
}
