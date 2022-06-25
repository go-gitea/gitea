// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package translation

import (
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation/i18n"

	"golang.org/x/text/language"
)

// Locale represents an interface to translation
type Locale interface {
	Language() string
	Tr(string, ...interface{}) string
	TrN(cnt interface{}, key1, keyN string, args ...interface{}) string
}

// LangType represents a lang type
type LangType struct {
	Lang, Name string // these fields are used directly in templates: {{range .AllLangs}}{{.Lang}}{{.Name}}{{end}}
	Offset     int
}

var (
	matcher       language.Matcher
	allLangs      []*LangType
	allLangMap    map[string]*LangType
	supportedTags []language.Tag
)

// AllLangs returns all supported languages sorted by name
func AllLangs() []*LangType {
	return allLangs
}

// TryTr tries to do the translation, if no translation, it returns (format, false)
func TryTr(lang, format string, args ...interface{}) (string, bool) {
	s := i18n.DefaultLocales.Tr(lang, format, args...)
	// now the i18n library is not good enough and we can only use this hacky method to detect whether the transaction exists
	idx := strings.IndexByte(format, '.')
	defaultText := format
	if idx > 0 {
		defaultText = format[idx+1:]
	}
	return s, s != defaultText
}

// moveToFront moves needle to the front of haystack, in place if possible.
// Ref: https://github.com/golang/go/wiki/SliceTricks#move-to-front-or-prepend-if-not-present-in-place-if-possible
func moveToFront(needle string, haystack []string) []string {
	if len(haystack) != 0 && haystack[0] == needle {
		return haystack
	}
	prev := needle
	for i, elem := range haystack {
		switch {
		case i == 0:
			haystack[0] = needle
			prev = elem
		case elem == needle:
			haystack[i] = prev
			return haystack
		default:
			haystack[i] = prev
			prev = elem
		}
	}
	return append(haystack, prev)
}

// InitLocales loads the locales
func InitLocales() {
	i18n.ResetDefaultLocales()
	localeNames, err := options.Dir("locale")
	if err != nil {
		log.Fatal("Failed to list locale files: %v", err)
	}

	localFiles := make(map[string][]byte, len(localeNames))
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

	// Make sure en-US is always the first in the slice.
	setting.Names = moveToFront("en-US", setting.Names)

	for i := range setting.Names {
		key := "locale_" + setting.Langs[i] + ".ini"
		if err = i18n.DefaultLocales.AddLocaleByIni(setting.Langs[i], setting.Names[i], localFiles[key]); err != nil {
			log.Error("Failed to set messages to %s: %v", setting.Langs[i], err)
		}
	}

	if len(setting.Langs) != 0 {
		defaultLangName := setting.Langs[0]
		if defaultLangName != "en-US" {
			log.Info("Use the first locale (%s) in LANGS setting option as default", defaultLangName)
		}
		i18n.DefaultLocales.SetDefaultLang(defaultLangName)
	}

	langs, descs, offsets := i18n.DefaultLocales.ListLangNameDescOffsets()
	allLangs = make([]*LangType, 0, len(langs))
	allLangMap = map[string]*LangType{}
	for i, v := range langs {
		l := &LangType{v, descs[i], offsets[i]}
		allLangs = append(allLangs, l)
		allLangMap[v] = l
	}

	// Sort languages case-insensitive according to their name - needed for the user settings
	sort.Slice(allLangs, func(i, j int) bool {
		return strings.ToLower(allLangs[i].Name) < strings.ToLower(allLangs[j].Name)
	})
}

// Match matches accept languages
func Match(tags ...language.Tag) language.Tag {
	_, i, _ := matcher.Match(tags...)
	return supportedTags[i]
}

// locale represents the information of localization.
type locale struct {
	Lang, LangName string // these fields are used directly in templates: .i18n.Lang
	// Stores the offset for the locale. The value is utilized by the 'TrOffset' function
	// to change the translation key's found index (for the default language) to the locale's index.
	Offset int
}

// NewLocale return a locale
func NewLocale(lang string) Locale {
	langName := "unknown"
	offset := 0
	if l, ok := allLangMap[lang]; ok {
		langName = l.Name
		offset = l.Offset
	}
	return &locale{
		Lang:     lang,
		LangName: langName,
		Offset:   offset,
	}
}

func (l *locale) Language() string {
	return l.Lang
}

// Tr translates content to target language.
func (l *locale) Tr(format string, args ...interface{}) string {
	if setting.IsProd {
		return i18n.TrOffset(l.Offset, format, args...)
	}

	// in development, we should show an error if a translation key is missing
	s, ok := TryTr(l.Lang, format, args...)
	if !ok {
		log.Error("missing i18n translation key: %q", format)
	}
	return s
}

// Language specific rules for translating plural texts
var trNLangRules = map[string]func(int64) int{
	// the default rule is "en-US" if a language isn't listed here
	"en-US": func(cnt int64) int {
		if cnt == 1 {
			return 0
		}
		return 1
	},
	"lv-LV": func(cnt int64) int {
		if cnt%10 == 1 && cnt%100 != 11 {
			return 0
		}
		return 1
	},
	"ru-RU": func(cnt int64) int {
		if cnt%10 == 1 && cnt%100 != 11 {
			return 0
		}
		return 1
	},
	"zh-CN": func(cnt int64) int {
		return 0
	},
	"zh-HK": func(cnt int64) int {
		return 0
	},
	"zh-TW": func(cnt int64) int {
		return 0
	},
	"fr-FR": func(cnt int64) int {
		if cnt > -2 && cnt < 2 {
			return 0
		}
		return 1
	},
}

// TrN returns translated message for plural text translation
func (l *locale) TrN(cnt interface{}, key1, keyN string, args ...interface{}) string {
	var c int64
	if t, ok := cnt.(int); ok {
		c = int64(t)
	} else if t, ok := cnt.(int16); ok {
		c = int64(t)
	} else if t, ok := cnt.(int32); ok {
		c = int64(t)
	} else if t, ok := cnt.(int64); ok {
		c = t
	} else {
		return l.Tr(keyN, args...)
	}

	ruleFunc, ok := trNLangRules[l.Lang]
	if !ok {
		ruleFunc = trNLangRules["en-US"]
	}

	if ruleFunc(c) == 0 {
		return l.Tr(key1, args...)
	}
	return l.Tr(keyN, args...)
}
