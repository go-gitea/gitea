// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"

	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/translation/i18n"

	"golang.org/x/text/language"
)

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
		tags, _, _ := language.ParseAcceptLanguage(req.Header.Get("Accept-Language"))
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

// DeleteLocaleCookie convenience function to delete the locale cookie consistently
// Setting the lang cookie will trigger the middleware to reset the language to previous state.
func DeleteLocaleCookie(resp http.ResponseWriter) {
	SetSiteCookie(resp, "lang", "", -1)
}
