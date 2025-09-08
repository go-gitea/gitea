// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// defaultI18nLangNames must be a slice, we need the order
var defaultI18nLangNames = []string{
	"en-US", "English",
	"zh-CN", "简体中文",
	"zh-HK", "繁體中文（香港）",
	"zh-TW", "繁體中文（台灣）",
	"de-DE", "Deutsch",
	"fr-FR", "Français",
	"ga-IE", "Gaeilge",
	"nl-NL", "Nederlands",
	"lv-LV", "Latviešu",
	"ru-RU", "Русский",
	"uk-UA", "Українська",
	"ja-JP", "日本語",
	"es-ES", "Español",
	"pt-BR", "Português do Brasil",
	"pt-PT", "Português de Portugal",
	"pl-PL", "Polski",
	"bg-BG", "Български",
	"it-IT", "Italiano",
	"fi-FI", "Suomi",
	"tr-TR", "Türkçe",
	"cs-CZ", "Čeština",
	"sv-SE", "Svenska",
	"ko-KR", "한국어",
	"el-GR", "Ελληνικά",
	"fa-IR", "فارسی",
	"hu-HU", "Magyar nyelv",
	"id-ID", "Bahasa Indonesia",
	"ml-IN", "മലയാളം",
}

func defaultI18nLangs() (res []string) {
	for i := 0; i < len(defaultI18nLangNames); i += 2 {
		res = append(res, defaultI18nLangNames[i])
	}
	return res
}

func defaultI18nNames() (res []string) {
	for i := 0; i < len(defaultI18nLangNames); i += 2 {
		res = append(res, defaultI18nLangNames[i+1])
	}
	return res
}

var (
	// I18n settings
	Langs []string
	Names []string
)

func loadI18nFrom(rootCfg ConfigProvider) {
	Langs = rootCfg.Section("i18n").Key("LANGS").Strings(",")
	if len(Langs) == 0 {
		Langs = defaultI18nLangs()
	}
	Names = rootCfg.Section("i18n").Key("NAMES").Strings(",")
	if len(Names) == 0 {
		Names = defaultI18nNames()
	}
}
