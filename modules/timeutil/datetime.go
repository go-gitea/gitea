// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"fmt"
	"html"
	"html/template"
	"time"
)

// DateTime renders an absolute time HTML element by datetime.
func DateTime(format string, datetime any) template.HTML {
	if p, ok := datetime.(*time.Time); ok {
		datetime = *p
	}
	if p, ok := datetime.(*TimeStamp); ok {
		datetime = *p
	}
	switch v := datetime.(type) {
	case TimeStamp:
		datetime = v.AsTime()
	case int:
		datetime = TimeStamp(v).AsTime()
	case int64:
		datetime = TimeStamp(v).AsTime()
	}

	var datetimeEscaped, textEscaped string
	switch v := datetime.(type) {
	case nil:
		return "-"
	case string:
		datetimeEscaped = html.EscapeString(v)
		textEscaped = datetimeEscaped
	case time.Time:
		if v.IsZero() || v.Unix() == 0 {
			return "-"
		}
		datetimeEscaped = html.EscapeString(v.Format(time.RFC3339))
		if format == "full" {
			textEscaped = html.EscapeString(v.Format("2006-01-02 15:04:05 -07:00"))
		} else {
			textEscaped = html.EscapeString(v.Format("2006-01-02"))
		}
	default:
		panic(fmt.Sprintf("Unsupported time type %T", datetime))
	}

	switch format {
	case "short":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="%s">%s</relative-time>`, datetimeEscaped, textEscaped))
	case "long":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" year="numeric" month="long" day="numeric" weekday="" datetime="%s">%s</relative-time>`, datetimeEscaped, textEscaped))
	case "full":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" weekday="" year="numeric" month="short" day="numeric" hour="numeric" minute="numeric" second="numeric" datetime="%s">%s</relative-time>`, datetimeEscaped, textEscaped))
	}
	panic(fmt.Sprintf("Unsupported format %s", format))
}
