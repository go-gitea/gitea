// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package i18n

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/ini.v1"
)

var (
	ErrLocaleAlreadyExist = errors.New("lang already exists")

	DefaultLocales = NewLocaleStore()
)

type locale struct {
	store    *LocaleStore
	langName string
	messages *ini.File
}

type LocaleStore struct {
	// After initializing has finished, these fields are read-only.
	langNames              []string
	langDescs              []string
	langOffsets            []int
	offsetToTranslationMap []map[string]string
	localeMap              map[string]*locale
	defaultLang            string
}

func NewLocaleStore() *LocaleStore {
	return &LocaleStore{localeMap: make(map[string]*locale)}
}

// AddLocaleByIni adds locale by ini into the store
func (ls *LocaleStore) AddLocaleByIni(langName, langDesc string, localeFile interface{}, otherLocaleFiles ...interface{}) error {
	if _, ok := ls.localeMap[langName]; ok {
		return ErrLocaleAlreadyExist
	}
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
	}, localeFile, otherLocaleFiles...)
	if err == nil {
		// Common code between production and development.
		ls.langNames = append(ls.langNames, langName)
		ls.langDescs = append(ls.langDescs, langDesc)

		// Specify the offset for translationValues.
		ls.langOffsets = append(ls.langOffsets, len(ls.langOffsets))

		// Make a distinquishment between production and development.
		// For development, live-reload of the translation files is important.
		// For production, we can do some expensive work and then make the querying fast.
		if setting.IsProd {
			keyToValue := map[string]string{}
			// Go trough all keys and store key->value into a map.
			for _, section := range iniFile.Sections() {
				for _, key := range section.Keys() {
					keyToValue[strings.TrimPrefix(section.Name()+"."+key.Name(), "DEFAULT.")] = key.Value()
				}
			}
			// Append the key->value to the offsetToTranslationMap variable.
			ls.offsetToTranslationMap = append(ls.offsetToTranslationMap, keyToValue)

			// Help Go's GC.
			iniFile = nil

			// Specify the offset for translationValues.
			ls.langOffsets = append(ls.langOffsets, len(ls.langOffsets))

			// Stub info for `HasLang`
			ls.localeMap[langName] = nil
		} else {
			// Add the language to the localeMap.
			iniFile.BlockMode = false
			lc := &locale{store: ls, langName: langName, messages: iniFile}
			ls.localeMap[lc.langName] = lc
		}
	}
	return err
}

func (ls *LocaleStore) HasLang(langName string) bool {
	_, ok := ls.localeMap[langName]
	return ok
}

func (ls *LocaleStore) ListLangNameDescOffsets() (names, desc []string, offsets []int) {
	return ls.langNames, ls.langDescs, ls.langOffsets
}

// SetDefaultLang sets default language as a fallback
func (ls *LocaleStore) SetDefaultLang(lang string) {
	ls.defaultLang = lang
}

// Tr translates content to target language. fall back to default language.
func (ls *LocaleStore) Tr(lang, trKey string, trArgs ...interface{}) string {
	l, ok := ls.localeMap[lang]
	if !ok {
		l, ok = ls.localeMap[ls.defaultLang]
	}
	if ok {
		return l.Tr(trKey, trArgs...)
	}
	return trKey
}

// Tr translates content to locale language. fall back to default language.
func (l *locale) Tr(trKey string, trArgs ...interface{}) string {
	var section string

	idx := strings.IndexByte(trKey, '.')
	if idx > 0 {
		section = trKey[:idx]
		trKey = trKey[idx+1:]
	}

	trMsg := trKey
	if trIni, err := l.messages.Section(section).GetKey(trKey); err == nil {
		trMsg = trIni.Value()
	} else if l.store.defaultLang != "" && l.langName != l.store.defaultLang {
		// try to fall back to default
		if defaultLocale, ok := l.store.localeMap[l.store.defaultLang]; ok {
			if trIni, err = defaultLocale.messages.Section(section).GetKey(trKey); err == nil {
				trMsg = trIni.Value()
			}
		}
	}

	if len(trArgs) > 0 {
		fmtArgs := make([]interface{}, 0, len(trArgs))
		for _, arg := range trArgs {
			val := reflect.ValueOf(arg)
			if val.Kind() == reflect.Slice {
				// before, it can accept Tr(lang, key, a, [b, c], d, [e, f]) as Sprintf(msg, a, b, c, d, e, f), it's an unstable behavior
				// now, we restrict the strange behavior and only support:
				// 1. Tr(lang, key, [slice-items]) as Sprintf(msg, items...)
				// 2. Tr(lang, key, args...) as Sprintf(msg, args...)
				if len(trArgs) == 1 {
					for i := 0; i < val.Len(); i++ {
						fmtArgs = append(fmtArgs, val.Index(i).Interface())
					}
				} else {
					log.Error("the args for i18n shouldn't contain uncertain slices, key=%q, args=%v", trKey, trArgs)
					break
				}
			} else {
				fmtArgs = append(fmtArgs, arg)
			}
		}
		return fmt.Sprintf(trMsg, fmtArgs...)
	}
	return trMsg
}

func ResetDefaultLocales() {
	DefaultLocales = NewLocaleStore()
}

// Tr use default locales to translate content to target language.
func Tr(lang, trKey string, trArgs ...interface{}) string {
	return DefaultLocales.Tr(lang, trKey, trArgs...)
}

func TrOffset(offset int, trKey string, trArgs ...interface{}) string {
	languageTranslationMap := DefaultLocales.offsetToTranslationMap[offset]
	trMsg, ok := languageTranslationMap[trKey]
	if !ok {
		return trKey
	}

	if len(trArgs) > 0 {
		fmtArgs := make([]interface{}, 0, len(trArgs))
		for _, arg := range trArgs {
			val := reflect.ValueOf(arg)
			if val.Kind() == reflect.Slice {
				// before, it can accept Tr(lang, key, a, [b, c], d, [e, f]) as Sprintf(msg, a, b, c, d, e, f), it's an unstable behavior
				// now, we restrict the strange behavior and only support:
				// 1. Tr(lang, key, [slice-items]) as Sprintf(msg, items...)
				// 2. Tr(lang, key, args...) as Sprintf(msg, args...)
				if len(trArgs) == 1 {
					for i := 0; i < val.Len(); i++ {
						fmtArgs = append(fmtArgs, val.Index(i).Interface())
					}
				} else {
					log.Error("the args for i18n shouldn't contain uncertain slices, key=%q, args=%v", trKey, trArgs)
					break
				}
			} else {
				fmtArgs = append(fmtArgs, arg)
			}
		}
		return fmt.Sprintf(trMsg, fmtArgs...)
	}
	return trMsg
}
