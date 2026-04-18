// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"html"
	"html/template"
)

// EscapeFlashErrorString will escape the flash error string
// Maybe do more sanitization in the future, e.g.: hide sensitive information, etc.
func EscapeFlashErrorString(x string) template.HTML {
	return template.HTML(html.EscapeString(x))
}
