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

var (
	matcher language.Matcher
)

// InitLocales loads the locales
func InitLocales() {
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

	// These codes will be used once macaron removed
	tags := make([]language.Tag, len(setting.Langs))
	for i, lang := range setting.Langs {
		tags[i] = language.Raw.Make(lang)
	}

	matcher = language.NewMatcher(tags)
	for i := range setting.Names {
		key := "locale_" + setting.Langs[i] + ".ini"
		if err := i18n.SetMessage(setting.Langs[i], localFiles[key]); err != nil {
			log.Fatal("Failed to set messages to %s", setting.Langs[i])
		}
	}
	i18n.SetDefaultLang("en-US")
}

// Match matches accept languages
func Match(tags ...language.Tag) (tag language.Tag, index int, c language.Confidence) {
	return matcher.Match(tags...)
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
