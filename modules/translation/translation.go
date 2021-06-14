// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package translation

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/i18n"
	"golang.org/x/text/language"
)

// Locale represents an interface to translation
type Locale interface {
	Language() string
	Tr(string, ...interface{}) string
}

// LangType represents a lang type
type LangType struct {
	Lang, Name string
}

var (
	matcher       language.Matcher
	allLangs      []LangType
	supportedTags []language.Tag
)

// AllLangs returns all supported langauages
func AllLangs() []LangType {
	return allLangs
}

// InitLocales loads the locales
func InitLocales() {
	i18n.Reset()
	localeNames, err := options.Dir("locale")
	if err != nil {
		log.Fatal("Failed to list locale files: %v", err)
	}

	localFiles := make(map[string][]byte)
	for _, name := range localeNames {
		localFiles[name], err = options.Locale(name)
		if err != nil {
			log.Fatal("Failed to load %s locale file. %v", name, err)
		}
	}

	supportedTags = make([]language.Tag, len(setting.Langs))
	for i, lang := range setting.Langs {
		supportedTags[i] = language.Raw.Make(lang)
	}

	matcher = language.NewMatcher(supportedTags)
	for i := range setting.Names {
		key := "locale_" + setting.Langs[i] + ".ini"
		if err = i18n.SetMessageWithDesc(setting.Langs[i], setting.Names[i], localFiles[key]); err != nil {
			log.Error("Failed to set messages to %s: %v", setting.Langs[i], err)
		}
	}
	i18n.SetDefaultLang("en-US")

	allLangs = make([]LangType, 0, i18n.Count()-1)
	langs := i18n.ListLangs()
	names := i18n.ListLangDescs()
	for i, v := range langs {
		allLangs = append(allLangs, LangType{v, names[i]})
	}
}

// Match matches accept languages
func Match(tags ...language.Tag) language.Tag {
	_, i, _ := matcher.Match(tags...)
	return supportedTags[i]
}

// locale represents the information of localization.
type locale struct {
	Lang string
}

// NewLocale return a locale
func NewLocale(lang string) Locale {
	return &locale{
		Lang: lang,
	}
}

func (l *locale) Language() string {
	return l.Lang
}

// Tr translates content to target language.
func (l *locale) Tr(format string, args ...interface{}) string {
	return i18n.Tr(l.Lang, format, args...)
}
