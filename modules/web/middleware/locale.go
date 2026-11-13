// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"

	"gitea.dev/modules/translation"
	"gitea.dev/modules/translation/i18n"

	"golang.org/x/text/language"
)

// maxAcceptLanguageLen bounds the Accept-Language header before it reaches
// language.ParseAcceptLanguage. That parser has quadratic-time behavior on long
// malformed inputs, and its built-in guard only counts "-" separators while the
// scanner treats "_" as an alias for "-", so a "_"-heavy header slips past the
// guard and burns CPU. Only the leading (highest-priority) languages are used, so
// truncating a longer header is safe.
const maxAcceptLanguageLen = 200

// parseAcceptLanguage parses the Accept-Language header after bounding its length
// to avoid a quadratic-time DoS on attacker-controlled input.
func parseAcceptLanguage(header string) []language.Tag {
	if len(header) > maxAcceptLanguageLen {
		header = header[:maxAcceptLanguageLen]
	}
	tags, _, _ := language.ParseAcceptLanguage(header)
	return tags
}

// Locale handle locale
func Locale(resp http.ResponseWriter, req *http.Request) translation.Locale {
	// 1. Check URL arguments.
	lang := req.URL.Query().Get("lang")
	changeLang := lang != ""

	// 2. Get language information from cookies.
	if len(lang) == 0 {
		ck, _ := req.Cookie("lang")
		if ck != nil {
			lang = ck.Value
		}
	}

	// Check again in case someone changes the supported language list.
	if lang != "" && !i18n.DefaultLocales.HasLang(lang) {
		lang = ""
		changeLang = false
	}

	// 3. Get language information from 'Accept-Language'.
	// The first element in the list is chosen to be the default language automatically.
	if len(lang) == 0 {
		tags := parseAcceptLanguage(req.Header.Get("Accept-Language"))
		tag := translation.Match(tags...)
		lang = tag.String()
	}

	if changeLang {
		SetLocaleCookie(resp, lang, 1<<31-1)
	}

	return translation.NewLocale(lang)
}

// SetLocaleCookie convenience function to set the locale cookie consistently
func SetLocaleCookie(resp http.ResponseWriter, lang string, maxAge int) {
	SetSiteCookie(resp, "lang", lang, maxAge)
}
