// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"fmt"
	"html"
	"html/template"
)

// MockLocale provides a mocked locale without any translations
type MockLocale struct {
	Lang, LangName string // these fields are used directly in templates: ctx.Locale.Lang
}

var _ Locale = (*MockLocale)(nil)

func (l MockLocale) Language() string {
	return "en"
}

func (l MockLocale) TrString(format string, args ...any) (ret string) {
	ret = format + ":"
	for _, arg := range args {
		// usually there is no arg or at most 1-2 args, so a simple string concatenation is more efficient
		switch v := arg.(type) {
		case string:
			ret += v + ","
		default:
			ret += fmt.Sprint(v) + ","
		}
	}
	return ret[:len(ret)-1]
}

func (l MockLocale) Tr(format string, args ...any) (ret template.HTML) {
	ret = template.HTML(html.EscapeString(format)) + ":"
	for _, arg := range args {
		// usually there is no arg or at most 1-2 args, so a simple string concatenation is more efficient
		switch v := arg.(type) {
		case template.HTML:
			ret += v + ","
		case string:
			ret += template.HTML(html.EscapeString(v)) + ","
		default:
			ret += template.HTML(html.EscapeString(fmt.Sprint(v))) + ","
		}
	}
	return ret[:len(ret)-1]
}

func (l MockLocale) TrN(cnt any, key1, keyN string, args ...any) template.HTML {
	return l.Tr(key1, args...)
}

func (l MockLocale) PrettyNumber(v any) string {
	return fmt.Sprint(v)
}
