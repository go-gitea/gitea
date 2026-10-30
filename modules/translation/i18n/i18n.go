// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package i18n

import (
	"html/template"
	"io"
)

var DefaultLocales = NewLocaleStore()

type Locale interface {
	// TrString translates a given key and arguments for a language
	TrString(trKey string, trArgs ...any) string
	// TrHTML translates a given key and arguments for a language, string arguments are escaped to HTML
	TrHTML(trKey string, trArgs ...any) template.HTML
	// HasKey reports if a locale has a translation for a given key
	HasKey(trKey string) bool
	// TrPluralString translates a key by the CLDR plural category of cnt, see TrPluralString in localestore.go
	TrPluralString(trKey string, cnt int64, trArgs ...any) string
	// TrPluralHTML is like TrPluralString, string arguments are escaped to HTML
	TrPluralHTML(trKey string, cnt int64, trArgs ...any) template.HTML
	// HasPluralKey reports if this locale (without fallback) defines CLDR plural forms for a given key
	HasPluralKey(trKey string) bool
	// HasOwnKey reports if this locale itself (without default-language fallback) has a translation for a given key
	HasOwnKey(trKey string) bool
	// PluralForms returns the raw messages by CLDR category ("one", "few", ...) of a plural key,
	// taken from this locale, or from the default locale if this locale has none
	PluralForms(trKey string) map[string]string
}

// LocaleTranslation represents an interface to translation
type LocaleTranslation interface {
	Language() string
	HasKey(trKey string) bool
	TrString(string, ...any) string

	Tr(key string, args ...any) template.HTML
	TrN(cnt any, key1, keyN string, args ...any) template.HTML
	// TrPlural translates by CLDR plural category of cnt (any integer type), falling back to legacy "<key>_1"/"<key>_n" keys
	TrPlural(key string, cnt any, args ...any) template.HTML
	TrPluralString(key string, cnt any, args ...any) string
	// TrPluralForms returns a JSON object of category -> raw message, for the frontend to pick by Intl.PluralRules
	TrPluralForms(key string) string
	HasPluralKey(key string) bool

	PrettyNumber(v any) string
}

// LocaleStore provides the functions common to all locale stores
type LocaleStore interface {
	io.Closer

	// SetDefaultLang sets the default language to fall back to
	SetDefaultLang(lang string)
	// ListLangNameDesc provides paired slices of language names to descriptors
	ListLangNameDesc() (names, desc []string)
	// Locale return the locale for the provided language or the default language if not found
	Locale(langName string) (Locale, bool)
	// HasLang returns whether a given language is present in the store
	HasLang(langName string) bool
	// AddLocaleByJSON adds a new language to the store
	AddLocaleByJSON(langName, langDesc string, source, moreSource []byte) error
}

// ResetDefaultLocales resets the current default locales
// NOTE: this is not synchronized
func ResetDefaultLocales() {
	if DefaultLocales != nil {
		_ = DefaultLocales.Close()
	}
	DefaultLocales = NewLocaleStore()
}

// GetLocale returns the locale from the default locales
func GetLocale(lang string) (Locale, bool) {
	return DefaultLocales.Locale(lang)
}
