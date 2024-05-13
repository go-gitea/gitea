// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"fmt"
	"html/template"
	"strings"
)

// MockLocale provides a mocked locale without any translations
type MockLocale struct {
	Lang, LangName string // these fields are used directly in templates: ctx.Locale.Lang
}

var _ Locale = (*MockLocale)(nil)

func (l MockLocale) Language() string {
	return "en"
}

func (l MockLocale) TrString(s string, args ...any) string {
	return sprintAny(s, args...)
}

func (l MockLocale) Tr(s string, args ...any) template.HTML {
	return template.HTML(sprintAny(s, args...))
}

func (l MockLocale) TrN(cnt any, key1, keyN string, args ...any) template.HTML {
	return template.HTML(sprintAny(key1, args...))
}

func (l MockLocale) PrettyNumber(v any) string {
	return fmt.Sprint(v)
}

func sprintAny(s string, args ...any) string {
	if len(args) == 0 {
		return s
	}
	return s + ":" + fmt.Sprintf(strings.Repeat(",%v", len(args))[1:], args...)
}
