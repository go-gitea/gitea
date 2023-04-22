// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timeutil

import (
	"fmt"
	"html/template"
)

// DateTime renders an absolute time HTML given a time as a string
func DateTime(format, datetime, fallback string) template.HTML {
	switch format {
	case "short":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" year="numeric" month="short" day="numeric" weekday="" datetime="%s">%s</relative-time>`, datetime, fallback))
	case "long":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" year="numeric" month="long" day="numeric" weekday="" datetime="%s">%s</relative-time>`, datetime, fallback))
	case "full":
		return template.HTML(fmt.Sprintf(`<relative-time format="datetime" weekday="" year="numeric" month="short" day="numeric" hour="numeric" minute="numeric" second="numeric" datetime="%s">%s</relative-time>`, datetime, fallback))
	}
	return template.HTML("error in DateTime")
}
