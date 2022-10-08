// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"fmt"
	"strings"
	"text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/translation/i18n/plurals"

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
func (store *localeStore) AddLocaleByIni(langName, langDesc string, source, moreSource []byte) error {
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
	}, source, moreSource)
	if err != nil {
		return fmt.Errorf("unable to load ini: %w", err)
	}
	iniFile.BlockMode = false

	for _, section := range iniFile.Sections() {
		for _, key := range section.Keys() {

			// Create a translation key for this section/key pair
			var trKey string
			if section.Name() == "" || section.Name() == "DEFAULT" {
				trKey = key.Name()
			} else {
				trKey = section.Name() + "." + key.Name()
			}

			// Look-up an idx for the key in the "global" key to idx map
			idx, ok := store.trKeyToIdxMap[trKey]
			if !ok {
				idx = len(store.trKeyToIdxMap)
				store.trKeyToIdxMap[trKey] = idx
			}

			// Store this value
			l.idxToMsgMap[idx] = key.Value()

			// Now handle plurals & ordinals
			ruletypes := []plurals.RuleType{plurals.Cardinal, plurals.Ordinal}
			for i, suffix := range []string{"_plural", "_ordinal"} {
				if !strings.HasSuffix(trKey, suffix) {
					continue
				}

				tmpl := template.New("")
				tmpl, err := tmpl.Parse(key.Value())
				if err != nil {
					log.Error("Misformatted key %s in %s: %v", trKey, l.langName, err)
					continue
				}

				pluralRules := plurals.DefaultRules.RuleByType(ruletypes[i], l.langName)

				for form := range pluralRules.PluralForms {
					formKey := trKey + "_" + string(form)

					// Get an idx for this new key
					idx, ok := store.trKeyToIdxMap[formKey]
					if !ok {
						idx = len(store.trKeyToIdxMap)
						store.trKeyToIdxMap[formKey] = idx
					}

					// Allow for already added explicit non-templated variants
					// (Later keys may just override our generated keys and that's fine)
					if _, ok := l.idxToMsgMap[idx]; ok {
						continue
					}

					// Otherwise generate from the template with the form
					sb := &strings.Builder{}
					err = tmpl.Execute(sb, form)
					if err != nil {
						log.Error("Misformatted key %s in %s: %v", trKey, l.langName, err)
						continue
					}

					l.idxToMsgMap[idx] = sb.String()
				}
			}

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
