// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"io"
)

var DefaultLocales = NewLocaleStore()

type Locale interface {
	// Tr translates a given key and arguments for a language
	Tr(trKey string, trArgs ...any) string
	// Has reports if a locale has a translation for a given key
	Has(trKey string) bool
}

// LocaleStore provides the functions common to all locale stores
type LocaleStore interface {
	io.Closer

	// Tr translates a given key and arguments for a language
	Tr(lang, trKey string, trArgs ...any) string
	// Has reports if a locale has a translation for a given key
	Has(lang, trKey string) bool
	// SetDefaultLang sets the default language to fall back to
	SetDefaultLang(lang string)
	// ListLangNameDesc provides paired slices of language names to descriptors
	ListLangNameDesc() (names, desc []string)
	// Locale return the locale for the provided language or the default language if not found
	Locale(langName string) (Locale, bool)
	// HasLang returns whether a given language is present in the store
	HasLang(langName string) bool
	// AddLocaleByIni adds a new language to the store
	AddLocaleByIni(langName, langDesc string, source, moreSource []byte) error
}

// ResetDefaultLocales resets the current default locales
// NOTE: this is not synchronized
func ResetDefaultLocales() {
	if DefaultLocales != nil {
		_ = DefaultLocales.Close()
	}
	DefaultLocales = NewLocaleStore()
}

// GetLocales returns the locale from the default locales
func GetLocale(lang string) (Locale, bool) {
	return DefaultLocales.Locale(lang)
}
