// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"html/template"
	"io"
	"strings"

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

// EscapeControlHTML escapes the Unicode control sequences in a provided html document
func EscapeControlHTML(html template.HTML, locale translation.Locale, opts ...EscapeOptions) (escaped *EscapeStatus, output template.HTML) {
	if !setting.UI.AmbiguousUnicodeDetection {
		return &EscapeStatus{}, html
	}
	w := &htmlutil.HTMLBuilder{}
	escaped, _ = EscapeControlReader(strings.NewReader(string(html)), w, locale, opts...)
	return escaped, w.HTMLString()
}

// EscapeControlReader escapes the Unicode control sequences in a provided reader of HTML content and writer in a locale and returns the findings as an EscapeStatus
func EscapeControlReader(reader io.Reader, writer htmlutil.HTMLWriter, locale translation.Locale, opts ...EscapeOptions) (*EscapeStatus, error) {
	if !setting.UI.AmbiguousUnicodeDetection {
		_, err := io.Copy(writer.OriginWriter(), reader)
		return &EscapeStatus{}, err
	}
	return escapeStream(locale, reader, writer.OriginWriter(), opts...)
}
