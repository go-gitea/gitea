// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"html/template"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/translation"
)

type EscapeOptions struct {
	Allowed map[rune]bool
}

func AllowRuneNBSP() map[rune]bool {
	return map[rune]bool{0xa0: true}
}

func EscapeOptionsForView() EscapeOptions {
	return EscapeOptions{
		// it's safe to see NBSP in the view, but maybe not in the diff
		Allowed: AllowRuneNBSP(),
	}
}

var asciiSafeBytes = func() (ret [256]bool) {
	for c := range ret {
		ret[c] = (c >= 0x20 && c < 0x7f) || c == '\t' || c == '\n'
	}
	return ret
}()

func isStringAsciiSafe(str string) bool {
	for i := range len(str) {
		if !asciiSafeBytes[str[i]] {
			return false
		}
	}
	return true
}

// EscapeControlHTML escapes the Unicode control sequences in a provided html document
func EscapeControlHTML(html template.HTML, locale translation.Locale, opts ...EscapeOptions) (escaped *EscapeStatus, output template.HTML) {
	if !setting.UI.AmbiguousUnicodeDetection || isStringAsciiSafe(string(html)) { // most code lines, nothing in them can need escaping
		return &EscapeStatus{}, html
	}
	escaper := &htmlEscaper{escaped: &EscapeStatus{}, locale: locale, ambiguousTables: AmbiguousTablesForLocale(locale)}
	if len(opts) > 0 {
		escaper.allowed = opts[0].Allowed
	}
	return escaper.escaped, template.HTML(escaper.escape(string(html)))
}
