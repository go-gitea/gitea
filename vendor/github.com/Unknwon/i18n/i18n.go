// Copyright 2013 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package i18n is for app Internationalization and Localization.
package i18n

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/ini.v1"
)

var (
	ErrLangAlreadyExist = errors.New("Lang already exists")

	locales = &localeStore{store: make(map[string]*locale)}
)

type locale struct {
	id       int
	lang     string
	langDesc string
	message  *ini.File
}

type localeStore struct {
	langs       []string
	langDescs   []string
	store       map[string]*locale
	defaultLang string
}

// Get target language string
func (d *localeStore) Get(lang, section, format string) (string, bool) {
	if locale, ok := d.store[lang]; ok {
		if key, err := locale.message.Section(section).GetKey(format); err == nil {
			return key.Value(), true
		}
	}

	if len(d.defaultLang) > 0 && lang != d.defaultLang {
		return d.Get(d.defaultLang, section, format)
	}

	return "", false
}

func (d *localeStore) Add(lc *locale) bool {
	if _, ok := d.store[lc.lang]; ok {
		return false
	}

	lc.id = len(d.langs)
	d.langs = append(d.langs, lc.lang)
	d.langDescs = append(d.langDescs, lc.langDesc)
	d.store[lc.lang] = lc

	return true
}

func (d *localeStore) Reload(langs ...string) (err error) {
	if len(langs) == 0 {
		for _, lc := range d.store {
			if err = lc.message.Reload(); err != nil {
				return err
			}
		}
	} else {
		for _, lang := range langs {
			if lc, ok := d.store[lang]; ok {
				if err = lc.message.Reload(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SetDefaultLang sets default language which is a indicator that
// when target language is not found, try find in default language again.
func SetDefaultLang(lang string) {
	locales.defaultLang = lang
}

// ReloadLangs reloads locale files.
func ReloadLangs(langs ...string) error {
	return locales.Reload(langs...)
}

// Count returns number of languages that are registered.
func Count() int {
	return len(locales.langs)
}

// ListLangs returns list of all locale languages.
func ListLangs() []string {
	langs := make([]string, len(locales.langs))
	copy(langs, locales.langs)
	return langs
}

func ListLangDescs() []string {
	langDescs := make([]string, len(locales.langDescs))
	copy(langDescs, locales.langDescs)
	return langDescs
}

// IsExist returns true if given language locale exists.
func IsExist(lang string) bool {
	_, ok := locales.store[lang]
	return ok
}

// IndexLang returns index of language locale,
// it returns -1 if locale not exists.
func IndexLang(lang string) int {
	if lc, ok := locales.store[lang]; ok {
		return lc.id
	}
	return -1
}

// GetLangByIndex return language by given index.
func GetLangByIndex(index int) string {
	if index < 0 || index >= len(locales.langs) {
		return ""
	}
	return locales.langs[index]
}

func GetDescriptionByIndex(index int) string {
	if index < 0 || index >= len(locales.langDescs) {
		return ""
	}

	return locales.langDescs[index]
}

func GetDescriptionByLang(lang string) string {
	return GetDescriptionByIndex(IndexLang(lang))
}

func SetMessageWithDesc(lang, langDesc string, localeFile interface{}, otherLocaleFiles ...interface{}) error {
	message, err := ini.Load(localeFile, otherLocaleFiles...)
	if err == nil {
		message.BlockMode = false
		lc := new(locale)
		lc.lang = lang
		lc.langDesc = langDesc
		lc.message = message

		if locales.Add(lc) == false {
			return ErrLangAlreadyExist
		}
	}
	return err
}

// SetMessage sets the message file for localization.
func SetMessage(lang string, localeFile interface{}, otherLocaleFiles ...interface{}) error {
	return SetMessageWithDesc(lang, lang, localeFile, otherLocaleFiles...)
}

// Locale represents the information of localization.
type Locale struct {
	Lang string
}

// Tr translates content to target language.
func (l Locale) Tr(format string, args ...interface{}) string {
	return Tr(l.Lang, format, args...)
}

// Index returns lang index of LangStore.
func (l Locale) Index() int {
	return IndexLang(l.Lang)
}

// Tr translates content to target language.
func Tr(lang, format string, args ...interface{}) string {
	var section string
	parts := strings.SplitN(format, ".", 2)
	if len(parts) == 2 {
		section = parts[0]
		format = parts[1]
	}

	value, ok := locales.Get(lang, section, format)
	if ok {
		format = value
	}

	if len(args) > 0 {
		params := make([]interface{}, 0, len(args))
		for _, arg := range args {
			if arg != nil {
				val := reflect.ValueOf(arg)
				if val.Kind() == reflect.Slice {
					for i := 0; i < val.Len(); i++ {
						params = append(params, val.Index(i).Interface())
					}
				} else {
					params = append(params, arg)
				}
			}
		}
		return fmt.Sprintf(format, params...)
	}
	return format
}
