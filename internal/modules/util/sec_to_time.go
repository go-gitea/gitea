// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

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
func SecToTime(durationVal any) string {
	duration, _ := ToInt64(durationVal)

	formattedTime := ""

	// The following four variables are calculated by taking
	// into account the previously calculated variables, this avoids
	// pitfalls when using remainders. As that could lead to incorrect
	// results when the calculated number equals the quotient number.
	remainingDays := duration / (60 * 60 * 24)
	years := remainingDays / 365
	remainingDays -= years * 365
	months := remainingDays * 12 / 365
	remainingDays -= months * 365 / 12
	weeks := remainingDays / 7
	remainingDays -= weeks * 7
	days := remainingDays

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
