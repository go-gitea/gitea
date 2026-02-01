// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"testing"

	"golang.org/x/net/html"
)

func TestHTMLStreamerWriterEscapesAttributeQuotes(t *testing.T) {
	var buf bytes.Buffer
	streamer := &HTMLStreamerWriter{Writer: &buf}

	attrValue := `{"a":"b"}`
	if err := streamer.StartTag("div", html.Attribute{Key: "data-json", Val: attrValue}); err != nil {
		t.Fatalf("StartTag: %v", err)
	}

	got := buf.String()
	want := `<div data-json="{&#34;a&#34;:&#34;b&#34;}">`
	if got != want {
		t.Fatalf("unexpected HTML output: got %q want %q", got, want)
	}
}
