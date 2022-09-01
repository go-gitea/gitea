// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"fmt"

	"gopkg.in/ini.v1"
)

type localeStore struct {
	// After initializing has finished, these fields are read-only.
	langNames []string
	langDescs []string

	localeMap     map[string]*locale
	trKeyToIdxMap map[string]int

	defaultLang string
}

// NewLocaleStore creates a static locale store
func NewLocaleStore() LocaleStore {
	return &localeStore{localeMap: make(map[string]*locale), trKeyToIdxMap: make(map[string]int)}
}

// AddLocaleByIni adds locale by ini into the store
// if source is a string, then the file is loaded
// if source is a []byte, then the content is used
func (store *localeStore) AddLocaleByIni(langName, langDesc string, source interface{}) error {
	if _, ok := store.localeMap[langName]; ok {
		return ErrLocaleAlreadyExist
	}

	store.langNames = append(store.langNames, langName)
	store.langDescs = append(store.langDescs, langDesc)

	l := newLocale(store, langName)
	store.localeMap[l.langName] = l

	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
	}, source)
	if err != nil {
		return fmt.Errorf("unable to load ini: %w", err)
	}
	iniFile.BlockMode = false

	for _, section := range iniFile.Sections() {
		for _, key := range section.Keys() {
			var trKey string
			if section.Name() == "" || section.Name() == "DEFAULT" {
				trKey = key.Name()
			} else {
				trKey = section.Name() + "." + key.Name()
			}
			idx, ok := store.trKeyToIdxMap[trKey]
			if !ok {
				idx = len(store.trKeyToIdxMap)
				store.trKeyToIdxMap[trKey] = idx
			}
			l.idxToMsgMap[idx] = key.Value()
		}
	}
	iniFile = nil

	return nil
}

func (store *localeStore) HasLang(langName string) bool {
	_, ok := store.localeMap[langName]
	return ok
}

func (store *localeStore) ListLangNameDesc() (names, desc []string) {
	return store.langNames, store.langDescs
}

// SetDefaultLang sets default language as a fallback
func (store *localeStore) SetDefaultLang(lang string) {
	store.defaultLang = lang
}

// Tr translates content to target language. fall back to default language.
func (store *localeStore) Tr(lang, trKey string, trArgs ...interface{}) string {
	l, _ := store.Locale(lang)

	return l.Tr(trKey, trArgs...)
}

// Has returns whether the given language has a translation for the provided key
func (store *localeStore) Has(lang, trKey string) bool {
	l, _ := store.Locale(lang)

	return l.Has(trKey)
}

// Locale returns the locale for the lang or the default language
func (store *localeStore) Locale(lang string) (Locale, bool) {
	l, found := store.localeMap[lang]
	if !found {
		var ok bool
		l, ok = store.localeMap[store.defaultLang]
		if !ok {
			// no default - return an empty locale
			l = newLocale(store, "")
		}
	}
	return l, found
}

// Close implements io.Closer
func (store *localeStore) Close() error {
	return nil
}
