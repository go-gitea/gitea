// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"strings"
	"time"

	"gitea.dev/modules/translation"
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

// trUnit translates "n units". Locales not yet migrated to CLDR plural forms still use the legacy
// singular key for n == 1, otherwise the plural message is chosen by the CLDR category of n.
func trUnit(lang translation.Locale, n int64, pluralKey, singularKey string) string {
	if n == 1 && !lang.HasPluralKey(pluralKey) {
		return lang.TrString(singularKey)
	}
	return lang.TrPluralString(pluralKey, n, n)
}

func computeTimeDiffFloor(diff int64, lang translation.Locale) (int64, string) {
	if diff <= 0 {
		return 0, lang.TrString("tool.now")
	}

	var unit int64
	var pluralKey, singularKey string
	switch {
	case diff < 1*Minute:
		unit, pluralKey, singularKey = 1, "tool.seconds", "tool.1s"
	case diff < 1*Hour:
		unit, pluralKey, singularKey = Minute, "tool.minutes", "tool.1m"
	case diff < 1*Day:
		unit, pluralKey, singularKey = Hour, "tool.hours", "tool.1h"
	case diff < 1*Week:
		unit, pluralKey, singularKey = Day, "tool.days", "tool.1d"
	case diff < 1*Month:
		unit, pluralKey, singularKey = Week, "tool.weeks", "tool.1w"
	case diff < 1*Year:
		unit, pluralKey, singularKey = Month, "tool.months", "tool.1mon"
	default:
		unit, pluralKey, singularKey = Year, "tool.years", "tool.1y"
	}
	n := diff / unit
	return diff - n*unit, trUnit(lang, n, pluralKey, singularKey)
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
