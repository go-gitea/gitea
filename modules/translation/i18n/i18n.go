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

	"gopkg.in/ini.v1"
)

var (
	ErrLocaleAlreadyExist = errors.New("lang already exists")

	DefaultLocales = NewLocaleStore()
)

type locale struct {
	store    *LocaleStore
	langName string
	langDesc string
	messages *ini.File
}

type LocaleStore struct {
	// at the moment, all these fields are readonly after initialization
	langNames   []string
	langDescs   []string
	localeMap   map[string]*locale
	defaultLang string
}

func NewLocaleStore() *LocaleStore {
	return &LocaleStore{localeMap: make(map[string]*locale)}
}

// AddLocaleByIni adds locale by ini into the store
func (ls *LocaleStore) AddLocaleByIni(langName, langDesc string, source, moreSource []byte) error {
	if _, ok := ls.localeMap[langName]; ok {
		return ErrLocaleAlreadyExist
	}
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
	}, source, moreSource)
	if err == nil {
		iniFile.BlockMode = false
		lc := &locale{store: ls, langName: langName, langDesc: langDesc, messages: iniFile}
		ls.langNames = append(ls.langNames, lc.langName)
		ls.langDescs = append(ls.langDescs, lc.langDesc)
		ls.localeMap[lc.langName] = lc
	}
	return err
}

func (ls *LocaleStore) HasLang(langName string) bool {
	_, ok := ls.localeMap[langName]
	return ok
}

func (ls *LocaleStore) ListLangNameDesc() (names, desc []string) {
	return ls.langNames, ls.langDescs
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
