// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"html/template"
	"io"
	"sync"

	"gitea.dev/modules/htmlutil"
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

var asciiSafeBytes = sync.OnceValue(func() (ret [256]bool) {
	for c := range ret {
		ret[c] = (c >= 0x20 && c < 0x7f) || c == '\t' || c == '\n'
	}
	return ret
})

func isStringAsciiSafe(str string) bool {
	safeBytes := asciiSafeBytes()
	for i := range len(str) {
		if !safeBytes[str[i]] {
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

// EscapeControlReader escapes the Unicode control sequences in a provided reader of HTML content and writer in a locale and returns the findings as an EscapeStatus
func EscapeControlReader(reader io.Reader, writer htmlutil.HTMLWriter, locale translation.Locale, opts ...EscapeOptions) (*EscapeStatus, error) {
	if !setting.UI.AmbiguousUnicodeDetection {
		_, err := io.Copy(writer.OriginWriter(), reader)
		return &EscapeStatus{}, err
	}
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	escaped, output := EscapeControlHTML(template.HTML(buf), locale, opts...)
	_, err = io.WriteString(writer.OriginWriter(), string(output))
	return escaped, err
}
