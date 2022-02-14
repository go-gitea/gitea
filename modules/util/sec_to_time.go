// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// SecToTime converts an amount of seconds to a human-readable string (example: 66s -> 1min 6s)
func SecToTime(duration int64) string {
	seconds := duration % 60
	minutes := (duration / (60)) % 60
	hours := duration / (60 * 60) % 24
	days := duration / (60 * 60) / 24

	var hrs string

	if days > 1 {
		hrs = fmt.Sprintf("%d days", days)
	} else {
		hrs = fmt.Sprintf("%d day", days)
	}
	if hours > 0 {
		if days == 0 {
			hrs = fmt.Sprintf("%dh", hours)
		} else {
			hrs = fmt.Sprintf("%s %dh", hrs, hours)
		}
	}
	if minutes > 0 {
		if days == 0 && hours == 0 {
			hrs = fmt.Sprintf("%dmin", minutes)
		} else {
			hrs = fmt.Sprintf("%s %dmin", hrs, minutes)
		}
	}
	if seconds > 0 {
		if days == 0 && hours == 0 && minutes == 0 {
			hrs = fmt.Sprintf("%ds", seconds)
		} else {
			hrs = fmt.Sprintf("%s %ds", hrs, seconds)
		}
	}

	return hrs
}
