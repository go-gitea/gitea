// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// test if local file has format error

// +build ignore

package main

import (
	"errors"
	"fmt"

	"log"

	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"github.com/unknwon/i18n"
	"golang.org/x/text/language"
)

func main() {
	localeNames, err := options.Dir("locale")
	if err != nil {
		log.Fatal("Failed to list locale files:", err)
	}

	localFiles := make(map[string][]byte)
	for _, name := range localeNames {
		localFiles[name], err = options.Locale(name)
		if err != nil {
			log.Fatal(fmt.Sprintf("Failed to load %s locale file. %v", name, err))
		}
	}

	setting.Langs = []string{
		"en-US", "zh-CN", "zh-HK", "zh-TW", "de-DE", "fr-FR", "nl-NL", "lv-LV",
		"ru-RU", "uk-UA", "ja-JP", "es-ES", "pt-BR", "pt-PT", "pl-PL", "bg-BG",
		"it-IT", "fi-FI", "tr-TR", "cs-CZ", "sr-SP", "sv-SE", "ko-KR"}
	setting.Names = []string{"English", "简体中文", "繁體中文（香港）", "繁體中文（台灣）", "Deutsch",
		"français", "Nederlands", "latviešu", "русский", "Українська", "日本語",
		"español", "português do Brasil", "Português de Portugal", "polski", "български",
		"italiano", "suomi", "Türkçe", "čeština", "српски", "svenska", "한국어"}

	tags := make([]language.Tag, len(setting.Langs))
	for i, lang := range setting.Langs {
		tags[i] = language.Raw.Make(lang)
	}

	hasEerr := false
	for i := range setting.Names {
		key := "locale_" + setting.Langs[i] + ".ini"
		if err = i18n.SetMessageWithDesc(setting.Langs[i], setting.Names[i], localFiles[key]); err != nil {
			if errors.Is(err, i18n.ErrLangAlreadyExist) {
				// just log if lang is already loaded since we can not reload it
				log.Printf("Can not load language '%s' since already loaded\n", setting.Langs[i])
			} else {
				log.Printf("Failed to set messages to %s: %v\n", setting.Langs[i], err)
				hasEerr = true
			}
		}
	}

	if hasEerr {
		log.Fatal("some locale files has format error")
	}
}
