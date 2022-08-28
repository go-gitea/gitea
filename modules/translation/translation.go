// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package translation

import (
	"context"
	"sort"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation/i18n"
	"code.gitea.io/gitea/modules/watcher"

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
}

var (
	lock          *sync.RWMutex
	matcher       language.Matcher
	allLangs      []*LangType
	allLangMap    map[string]*LangType
	supportedTags []language.Tag
)

// AllLangs returns all supported languages sorted by name
func AllLangs() []*LangType {
	return allLangs
}

// InitLocales loads the locales
func InitLocales(ctx context.Context) {
	if lock != nil {
		lock.Lock()
		defer lock.Unlock()
	} else if !setting.IsProd && lock == nil {
		lock = &sync.RWMutex{}
	}

	refreshLocales := func() {
		i18n.ResetDefaultLocales()
		localeNames, err := options.Dir("locale")
		if err != nil {
			log.Fatal("Failed to list locale files: %v", err)
		}

		localFiles := make(map[string]interface{}, len(localeNames))
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
	}

	refreshLocales()

	langs, descs := i18n.DefaultLocales.ListLangNameDesc()
	allLangs = make([]*LangType, 0, len(langs))
	allLangMap = map[string]*LangType{}
	for i, v := range langs {
		l := &LangType{v, descs[i]}
		allLangs = append(allLangs, l)
		allLangMap[v] = l
	}

	// Sort languages case-insensitive according to their name - needed for the user settings
	sort.Slice(allLangs, func(i, j int) bool {
		return strings.ToLower(allLangs[i].Name) < strings.ToLower(allLangs[j].Name)
	})

	if !setting.IsProd {
		watcher.CreateWatcher(ctx, "Locales", &watcher.CreateWatcherOpts{
			PathsCallback: options.WalkLocales,
			BetweenCallback: func() {
				lock.Lock()
				defer lock.Unlock()
				refreshLocales()
			},
		})
	}
}

// Match matches accept languages
func Match(tags ...language.Tag) language.Tag {
	_, i, _ := matcher.Match(tags...)
	return supportedTags[i]
}

// locale represents the information of localization.
type locale struct {
	i18n.Locale
	Lang, LangName string // these fields are used directly in templates: .i18n.Lang
}

// NewLocale return a locale
func NewLocale(lang string) Locale {
	if lock != nil {
		lock.RLock()
		defer lock.RUnlock()
	}

	langName := "unknown"
	if l, ok := allLangMap[lang]; ok {
		langName = l.Name
	}
	i18nLocale, _ := i18n.GetLocale(lang)
	return &locale{
		Locale:   i18nLocale,
		Lang:     lang,
		LangName: langName,
	}
}

func (l *locale) Language() string {
	return l.Lang
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
