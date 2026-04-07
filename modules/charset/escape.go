// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"html/template"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
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
	sb := &strings.Builder{}
	escaped, _ = EscapeControlReader(strings.NewReader(string(html)), sb, locale, opts...) // err has been handled in EscapeControlReader
	return escaped, template.HTML(sb.String())
}

// EscapeControlReader escapes the Unicode control sequences in a provided reader of HTML content and writer in a locale and returns the findings as an EscapeStatus
func EscapeControlReader(reader io.Reader, writer io.Writer, locale translation.Locale, opts ...EscapeOptions) (*EscapeStatus, error) {
	return escapeStream(locale, reader, writer, opts...)
}
