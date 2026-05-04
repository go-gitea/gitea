// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

type DateUtils struct{}

func NewDateUtils() *DateUtils {
	return (*DateUtils)(nil) // the util is stateless, and we do not need to create an instance
}

// AbsoluteShort renders in "Jan 01, 2006" format
func (du *DateUtils) AbsoluteShort(time any) template.HTML {
	return dateTimeFormat("short", time)
}

// AbsoluteLong renders in "January 01, 2006" format
func (du *DateUtils) AbsoluteLong(time any) template.HTML {
	return dateTimeFormat("long", time)
}

// FullTime renders in "Jan 01, 2006 20:33:44" format
func (du *DateUtils) FullTime(time any) template.HTML {
	return dateTimeFormat("full", time)
}

func (du *DateUtils) TimeSince(time any) template.HTML {
	return TimeSince(time)
}

func (du *DateUtils) TimeSinceHeat(datetime any) string {
	return timeSinceHeatTo(datetime, time.Now())
}

func (du *DateUtils) TimeSinceOpacityClass(datetime any) string {
	return timeSinceOpacityClassTo(datetime, time.Now())
}

// ParseLegacy parses the datetime in legacy format, eg: "2016-01-02" in server's timezone.
// It shouldn't be used in new code. New code should use Time or TimeStamp as much as possible.
func (du *DateUtils) ParseLegacy(datetime string) time.Time {
	return parseLegacy(datetime)
}

func parseLegacy(datetime string) time.Time {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		t, _ = time.ParseInLocation(time.DateOnly, datetime, setting.DefaultUILocation)
	}
	return t
}

func anyToTime(value any) (t time.Time, isZero bool) {
	switch v := value.(type) {
	case nil:
		// it is zero
	case *time.Time:
		if v != nil {
			t = *v
		}
	case time.Time:
		t = v
	case timeutil.TimeStamp:
		t = v.AsTime()
	case timeutil.TimeStampNano:
		t = v.AsTime()
	case int:
		t = timeutil.TimeStamp(v).AsTime()
	case int64:
		t = timeutil.TimeStamp(v).AsTime()
	default:
		panic(fmt.Sprintf("Unsupported time type %T", value))
	}
	return t, t.IsZero() || t.Unix() == 0
}

func dateTimeFormat(format string, datetime any) template.HTML {
	t, isZero := anyToTime(datetime)
	if isZero {
		return "-"
	}
	var textEscaped string
	datetimeEscaped := html.EscapeString(t.Format(time.RFC3339))
	if format == "full" {
		textEscaped = html.EscapeString(t.Format("2006-01-02 15:04:05 -07:00"))
	} else {
		textEscaped = html.EscapeString(t.Format("2006-01-02"))
	}

	attrs := []string{`weekday=""`, `year="numeric"`}
	switch format {
	case "short", "long": // date only
		attrs = append(attrs, `threshold="P0Y"`, `month="`+format+`"`, `day="numeric"`, `prefix=""`)
	case "full": // full date including time
		attrs = append(attrs, `format="datetime"`, `month="short"`, `day="numeric"`, `hour="numeric"`, `minute="numeric"`, `second="numeric"`, `data-tooltip-content`, `data-tooltip-interactive="true"`)
	default:
		panic("Unsupported format " + format)
	}

	return template.HTML(fmt.Sprintf(`<relative-time %s datetime="%s">%s</relative-time>`, strings.Join(attrs, " "), datetimeEscaped, textEscaped))
}

func timeSinceTo(then any, now time.Time) template.HTML {
	thenTime, isZero := anyToTime(then)
	if isZero {
		return "-"
	}

	friendlyText := thenTime.Format("2006-01-02 15:04:05 -07:00")

	// document: https://github.com/github/relative-time-element
	attrs := `tense="past"`
	isFuture := now.Before(thenTime)
	if isFuture {
		attrs = `tense="future"`
	}

	// declare data-tooltip-content attribute to switch from "title" tooltip to "tippy" tooltip
	htm := fmt.Sprintf(`<relative-time prefix="" %s datetime="%s" data-tooltip-content data-tooltip-interactive="true">%s</relative-time>`,
		attrs, thenTime.Format(time.RFC3339), friendlyText)
	return template.HTML(htm)
}

func inverseLinearInterpolation(low, high, value float64) float64 {
	if low == high {
		return 0
	}
	return (value - low) / (high - low)
}

func timeSinceHeatTo(then any, now time.Time) string {
	thenTime, isZero := anyToTime(then)
	if isZero {
		return ""
	}

	const maxHeatWindow = 2_000_000_000.0
	const month = 30 * 24 * time.Hour

	age := now.Sub(thenTime)
	if age > 4*month {
		return ""
	}

	value := -float64(age.Milliseconds())
	interp := inverseLinearInterpolation(-maxHeatWindow, 0, value)
	if interp < 0 {
		interp = 0
	} else if interp > 1 {
		interp = 1
	}

	heatIndex := max(1, 10-int(interp*10))
	return strconv.Itoa(heatIndex)
}

func timeSinceOpacityClassTo(then any, now time.Time) string {
	thenTime, isZero := anyToTime(then)
	if isZero {
		return ""
	}

	const month = 30 * 24 * time.Hour
	age := now.Sub(thenTime)

	switch {
	case age > 12*month:
		return "repo-file-age--very-faded"
	case age > 4*month:
		return "repo-file-age--faded"
	default:
		return ""
	}
}

// TimeSince renders relative time HTML given a time
func TimeSince(then any) template.HTML {
	if setting.UI.PreferredTimestampTense == "absolute" {
		return dateTimeFormat("full", then)
	}
	return timeSinceTo(then, time.Now())
}
