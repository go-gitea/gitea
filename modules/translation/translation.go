// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package translation

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"

	macaron_i18n "gitea.com/macaron/i18n"
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
	/*tags := make([]language.Tag, len(setting.Langs))
	for i, lang := range setting.Langs {
		tags[i] = language.Raw.Make(lang)
	}
	matcher = language.NewMatcher(tags)
	for i, name := range setting.Names {
		i18n.SetMessage(setting.Langs[i], localFiles[name])
	}
	i18n.SetDefaultLang("en-US")*/

	// To be compatible with macaron, we now have to use macaron i18n, once macaron
	// removed, we can use i18n directly
	macaron_i18n.I18n(macaron_i18n.Options{
		SubURL:       setting.AppSubURL,
		Files:        localFiles,
		Langs:        setting.Langs,
		Names:        setting.Names,
		DefaultLang:  "en-US",
		Redirect:     false,
		CookieDomain: setting.SessionConfig.Domain,
	})
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
