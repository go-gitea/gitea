// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import "fmt"

// MockLocale provides a mocked locale without any translations
type MockLocale struct{}

var _ Locale = (*MockLocale)(nil)

func (l MockLocale) Language() string {
	return "en"
}

func (l MockLocale) Tr(s string, _ ...any) string {
	return s
}

func (l MockLocale) TrN(_cnt any, key1, _keyN string, _args ...any) string {
	return key1
}

func (l MockLocale) PrettyNumber(v any) string {
	return fmt.Sprint(v)
}
