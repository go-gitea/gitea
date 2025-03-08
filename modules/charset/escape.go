// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:generate go run invisible/generate.go -v -o ./invisible_gen.go

//go:generate go run ambiguous/generate.go -v -o ./ambiguous_gen.go ambiguous/ambiguous.json

package charset

import (
	"context"
	"html/template"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
)

// RuneNBSP is the codepoint for NBSP
const RuneNBSP = 0xa0

// EscapeControlHTML escapes the unicode control sequences in a provided html document
func EscapeControlHTML(ctx context.Context, html template.HTML, locale translation.Locale, allowed ...rune) (escaped *EscapeStatus, output template.HTML) {
	sb := &strings.Builder{}
	escaped, _ = EscapeControlReader(ctx, strings.NewReader(string(html)), sb, locale, allowed...) // err has been handled in EscapeControlReader
	return escaped, template.HTML(sb.String())
}

// EscapeControlReader escapes the unicode control sequences in a provided reader of HTML content and writer in a locale and returns the findings as an EscapeStatus
func EscapeControlReader(ctx context.Context, reader io.Reader, writer io.Writer, locale translation.Locale, allowed ...rune) (escaped *EscapeStatus, err error) {
	if !setting.Config().UI.AmbiguousUnicodeDetection.Value(ctx) {
		_, err = io.Copy(writer, reader)
		return &EscapeStatus{}, err
	}
	outputStream := &HTMLStreamerWriter{Writer: writer}
	streamer := NewEscapeStreamer(locale, outputStream, allowed...).(*escapeStreamer)

	if err = StreamHTML(reader, streamer); err != nil {
		streamer.escaped.HasError = true
		log.Error("Error whilst escaping: %v", err)
	}
	return streamer.escaped, err
}
