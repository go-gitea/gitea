// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"fmt"
	"html/template"
)

// MockLocale provides a mocked locale without any translations
type MockLocale struct{}

var _ Locale = (*MockLocale)(nil)

func (l MockLocale) Language() string {
	return "en"
}

func (l MockLocale) TrString(s string, _ ...any) string {
	return s
}

func (l MockLocale) Tr(s string, a ...any) template.HTML {
	return template.HTML(s)
}

func (l MockLocale) TrN(cnt any, key1, keyN string, args ...any) template.HTML {
	return template.HTML(key1)
}

func (l MockLocale) PrettyNumber(v any) string {
	return fmt.Sprint(v)
}
