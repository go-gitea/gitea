// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/translation"
)

// Seconds-based time units
const (
	Minute = 60
	Hour   = 60 * Minute
	Day    = 24 * Hour
	Week   = 7 * Day
	Month  = 30 * Day
	Year   = 12 * Month
)

func computeTimeDiffFloor(diff int64, lang translation.Locale) (int64, string) {
	var diffStr string
	switch {
	case diff <= 0:
		diff = 0
		diffStr = lang.TrString("tool.now")
	case diff < 2:
		diff = 0
		diffStr = lang.TrString("tool.1s")
	case diff < 1*Minute:
		diffStr = lang.TrString("tool.seconds", diff)
		diff = 0

	case diff < 2*Minute:
		diff -= 1 * Minute
		diffStr = lang.TrString("tool.1m")
	case diff < 1*Hour:
		diffStr = lang.TrString("tool.minutes", diff/Minute)
		diff -= diff / Minute * Minute

	case diff < 2*Hour:
		diff -= 1 * Hour
		diffStr = lang.TrString("tool.1h")
	case diff < 1*Day:
		diffStr = lang.TrString("tool.hours", diff/Hour)
		diff -= diff / Hour * Hour

	case diff < 2*Day:
		diff -= 1 * Day
		diffStr = lang.TrString("tool.1d")
	case diff < 1*Week:
		diffStr = lang.TrString("tool.days", diff/Day)
		diff -= diff / Day * Day

	case diff < 2*Week:
		diff -= 1 * Week
		diffStr = lang.TrString("tool.1w")
	case diff < 1*Month:
		diffStr = lang.TrString("tool.weeks", diff/Week)
		diff -= diff / Week * Week

	case diff < 2*Month:
		diff -= 1 * Month
		diffStr = lang.TrString("tool.1mon")
	case diff < 1*Year:
		diffStr = lang.TrString("tool.months", diff/Month)
		diff -= diff / Month * Month

	case diff < 2*Year:
		diff -= 1 * Year
		diffStr = lang.TrString("tool.1y")
	default:
		diffStr = lang.TrString("tool.years", diff/Year)
		diff -= (diff / Year) * Year
	}
	return diff, diffStr
}

// MinutesToFriendly returns a user-friendly string with number of minutes
// converted to hours and minutes.
func MinutesToFriendly(minutes int, lang translation.Locale) string {
	duration := time.Duration(minutes) * time.Minute
	return timeSincePro(time.Now().Add(-duration), time.Now(), lang)
}

func timeSincePro(then, now time.Time, lang translation.Locale) string {
	diff := now.Unix() - then.Unix()

	if then.After(now) {
		return lang.TrString("tool.future")
	}
	if diff == 0 {
		return lang.TrString("tool.now")
	}

	var timeStr, diffStr string
	for {
		if diff == 0 {
			break
		}

		diff, diffStr = computeTimeDiffFloor(diff, lang)
		timeStr += ", " + diffStr
	}
	return strings.TrimPrefix(timeStr, ", ")
}
