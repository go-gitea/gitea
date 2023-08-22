// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"fmt"
	"html/template"
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
		diffStr = lang.Tr("tool.now")
	case diff < 2:
		diff = 0
		diffStr = lang.Tr("tool.1s")
	case diff < 1*Minute:
		diffStr = lang.Tr("tool.seconds", diff)
		diff = 0

	case diff < 2*Minute:
		diff -= 1 * Minute
		diffStr = lang.Tr("tool.1m")
	case diff < 1*Hour:
		diffStr = lang.Tr("tool.minutes", diff/Minute)
		diff -= diff / Minute * Minute

	case diff < 2*Hour:
		diff -= 1 * Hour
		diffStr = lang.Tr("tool.1h")
	case diff < 1*Day:
		diffStr = lang.Tr("tool.hours", diff/Hour)
		diff -= diff / Hour * Hour

	case diff < 2*Day:
		diff -= 1 * Day
		diffStr = lang.Tr("tool.1d")
	case diff < 1*Week:
		diffStr = lang.Tr("tool.days", diff/Day)
		diff -= diff / Day * Day

	case diff < 2*Week:
		diff -= 1 * Week
		diffStr = lang.Tr("tool.1w")
	case diff < 1*Month:
		diffStr = lang.Tr("tool.weeks", diff/Week)
		diff -= diff / Week * Week

	case diff < 2*Month:
		diff -= 1 * Month
		diffStr = lang.Tr("tool.1mon")
	case diff < 1*Year:
		diffStr = lang.Tr("tool.months", diff/Month)
		diff -= diff / Month * Month

	case diff < 2*Year:
		diff -= 1 * Year
		diffStr = lang.Tr("tool.1y")
	default:
		diffStr = lang.Tr("tool.years", diff/Year)
		diff -= (diff / Year) * Year
	}
	return diff, diffStr
}

// MinutesToFriendly returns a user friendly string with number of minutes
// converted to hours and minutes.
func MinutesToFriendly(minutes int, lang translation.Locale) string {
	duration := time.Duration(minutes) * time.Minute
	return TimeSincePro(time.Now().Add(-duration), lang)
}

// TimeSincePro calculates the time interval and generate full user-friendly string.
func TimeSincePro(then time.Time, lang translation.Locale) string {
	return timeSincePro(then, time.Now(), lang)
}

func timeSincePro(then, now time.Time, lang translation.Locale) string {
	diff := now.Unix() - then.Unix()

	if then.After(now) {
		return lang.Tr("tool.future")
	}
	if diff == 0 {
		return lang.Tr("tool.now")
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

func timeSinceUnix(then, now time.Time, lang translation.Locale) template.HTML {
	friendlyText := then.Format("2006-01-02 15:04:05 -07:00")

	// document: https://github.com/github/relative-time-element
	attrs := `tense="past"`
	isFuture := now.Before(then)
	if isFuture {
		attrs = `tense="future"`
	}

	// declare data-tooltip-content attribute to switch from "title" tooltip to "tippy" tooltip
	htm := fmt.Sprintf(`<relative-time class="time-since" prefix="" %s datetime="%s" data-tooltip-content data-tooltip-interactive="true">%s</relative-time>`,
		attrs, then.Format(time.RFC3339), friendlyText)
	return template.HTML(htm)
}

// TimeSince renders relative time HTML given a time.Time
func TimeSince(then time.Time, lang translation.Locale) template.HTML {
	return timeSinceUnix(then, time.Now(), lang)
}

// TimeSinceUnix renders relative time HTML given a TimeStamp
func TimeSinceUnix(then TimeStamp, lang translation.Locale) template.HTML {
	return TimeSince(then.AsLocalTime(), lang)
}
