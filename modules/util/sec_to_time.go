// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"strings"
)

// SecToTime converts an amount of seconds to a human-readable string. E.g.
// 66s			-> 1 minute 6 seconds
// 52410s		-> 14 hours 33 minutes
// 563418		-> 6 days 12 hours
// 1563418		-> 2 weeks 4 days
// 3937125s     -> 1 month 2 weeks
// 45677465s	-> 1 year 6 months
//
// Magic numbers:
// 3600 = 60 * 60 (amount of seconds in a hour)
// 86400 = 60 * 60 * 24 (amount of seconds in a day)
func SecToTime(duration int64) string {
	formattedTime := ""
	years := (duration / 86400) / 365

	// The following three variables are calculated with taking
	// into account the previous calculated variables, this avoids
	// pitfalls when using remainders. As that could lead to incorrect
	// results when the calculated number equals the quotient number.
	months := (duration/86400 - years*365) / 30
	weeks := (duration/86400 - years*365 - months*30) / 7
	days := duration/86400 - years*365 - months*30 - weeks*7

	// The following three variables are calculated without depending
	// on the previous calculated variables.
	hours := (duration / 3600) % 24
	minutes := (duration / 60) % 60
	seconds := duration % 60

	// Extract only the relevant information of the time
	// If the time is greater than a year, it makes no sense to display seconds.
	switch {
	case years > 0:
		formattedTime = formatTime(years, "year", formattedTime)
		formattedTime = formatTime(months, "month", formattedTime)
	case months > 0:
		formattedTime = formatTime(months, "month", formattedTime)
		formattedTime = formatTime(weeks, "week", formattedTime)
	case weeks > 0:
		formattedTime = formatTime(weeks, "week", formattedTime)
		formattedTime = formatTime(days, "day", formattedTime)
	case days > 0:
		formattedTime = formatTime(days, "day", formattedTime)
		formattedTime = formatTime(hours, "hour", formattedTime)
	case hours > 0:
		formattedTime = formatTime(hours, "hour", formattedTime)
		formattedTime = formatTime(minutes, "minute", formattedTime)
	default:
		formattedTime = formatTime(minutes, "minute", formattedTime)
		formattedTime = formatTime(seconds, "second", formattedTime)
	}

	// The formatTime() function always appends a space at the end. This will be trimmed
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
