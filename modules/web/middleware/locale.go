// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"

	"code.gitea.io/gitea/modules/translation"

	"github.com/unknwon/i18n"
	"golang.org/x/text/language"
)

// Locale handle locale
func Locale(resp http.ResponseWriter, req *http.Request) translation.Locale {
	// 1. Check URL arguments.
	lang := req.URL.Query().Get("lang")
	var changeLang = lang != ""

	// 2. Get language information from cookies.
	if len(lang) == 0 {
		ck, _ := req.Cookie("lang")
		if ck != nil {
			lang = ck.Value
		}
	}

	// Check again in case someone modify by purpose.
	if lang != "" && !i18n.IsExist(lang) {
		lang = ""
		changeLang = false
	}

	// 3. Get language information from 'Accept-Language'.
	// The first element in the list is chosen to be the default language automatically.
	if len(lang) == 0 {
		tags, _, _ := language.ParseAcceptLanguage(req.Header.Get("Accept-Language"))
		tag, _, _ := translation.Match(tags...)
		lang = tag.String()
	}

	if changeLang {
		SetCookie(resp, "lang", lang, 1<<31-1)
	}

	return translation.NewLocale(lang)
}
