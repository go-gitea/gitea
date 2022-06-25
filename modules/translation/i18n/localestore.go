// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import "io"

// LocaleStore provides the functions common to all locale stores
type LocaleStore interface {
	io.Closer

	// Tr translates a given key and arguments for a language
	Tr(lang, trKey string, trArgs ...interface{}) string
	// SetDefaultLang sets the default language to fall back to
	SetDefaultLang(lang string)
	// ListLangNameDesc provides paired slices of language names to descriptors
	ListLangNameDesc() (names, desc []string)
	// HasLang returns whether a given language is present in the store
	HasLang(langName string) bool
	// AddLocaleByIni adds a new language to the store
	AddLocaleByIni(langName, langDesc string, source interface{}) error
}
