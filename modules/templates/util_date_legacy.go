// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"

	"code.gitea.io/gitea/modules/translation"
)

func dateTimeLegacy(format string, datetime any, _ ...string) template.HTML {
	panicIfDevOrTesting()
	if s, ok := datetime.(string); ok {
		datetime = parseLegacy(s)
	}
	return dateTimeFormat(format, datetime)
}

func timeSinceLegacy(time any, _ translation.Locale) template.HTML {
	panicIfDevOrTesting()
	return TimeSince(time)
}
