// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"fmt"
	"html"
	"html/template"
	"strings"
	"time"
)

// DateTime renders an absolute time HTML element by datetime.
func DateTime(format string, datetime any, extraAttrs ...string) template.HTML {
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

	attrs := make([]string, 0, 10+len(extraAttrs))
	attrs = append(attrs, extraAttrs...)
	attrs = append(attrs, `data-tooltip-content`, `data-tooltip-interactive="true"`)
	attrs = append(attrs, `format="datetime"`, `weekday=""`, `year="numeric"`)

	switch format {
	case "short":
		attrs = append(attrs, `month="short"`, `day="numeric"`)
	case "long":
		attrs = append(attrs, `month="long"`, `day="numeric"`)
	case "full":
		attrs = append(attrs, `month="short"`, `day="numeric"`, `hour="numeric"`, `minute="numeric"`, `second="numeric"`)
	default:
		panic(fmt.Sprintf("Unsupported format %s", format))
	}
	return template.HTML(fmt.Sprintf(`<relative-time %s datetime="%s">%s</relative-time>`, strings.Join(attrs, " "), datetimeEscaped, textEscaped))
}
