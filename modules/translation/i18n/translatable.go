// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import "fmt"

// Locale represents an interface to translation
type Locale interface {
	Tr(key string, args ...interface{}) string
	TrPlural(cnt interface{}, key string, args ...interface{}) string
}

// TranslatableFormatted structs provide their own translated string when formatted in translation
type TranslatableFormatted interface {
	TranslatedFormat(l Locale, s fmt.State, c rune)
}

// TranslatableStringer structs provide their own translated string when formatted as a string in translation
type TranslatableStringer interface {
	TranslatedString(l Locale) string
}

type formatWrapper struct {
	l Locale
	t TranslatableFormatted
}

func (f formatWrapper) Format(s fmt.State, c rune) {
	f.t.TranslatedFormat(f.l, s, c)
}

type stringWrapper struct {
	l Locale
	t TranslatableStringer
}

func (s stringWrapper) String() string {
	return s.t.TranslatedString(s.l)
}
