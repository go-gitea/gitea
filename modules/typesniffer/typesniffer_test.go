// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package typesniffer

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectContentTypeLongerThanSniffLen(t *testing.T) {
	// Pre-condition: Shorter than sniffLen detects SVG.
	assert.Equal(t, "image/svg+xml", DetectContentType([]byte(`<!-- Comment --><svg></svg>`)).contentType)
	// Longer than sniffLen detects something else.
	assert.NotEqual(t, "image/svg+xml", DetectContentType([]byte(`<!-- `+strings.Repeat("x", sniffLen)+` --><svg></svg>`)).contentType)
}

func TestIsTextFile(t *testing.T) {
	assert.True(t, DetectContentType([]byte{}).IsText())
	assert.True(t, DetectContentType([]byte("lorem ipsum")).IsText())
}

func TestIsSvgImage(t *testing.T) {
	assert.True(t, DetectContentType([]byte("<svg></svg>")).IsSvgImage())
	assert.True(t, DetectContentType([]byte("    <svg></svg>")).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<svg width="100"></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte("<svg/>")).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?><svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<!-- Comment -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<!-- Multiple -->
	<!-- Comments -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<!-- Multiline
	Comment -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1 Basic//EN"
	"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11-basic.dtd">
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Comment -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multiple -->
	<!-- Comments -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- Multline
	Comment -->
	<svg></svg>`)).IsSvgImage())
	assert.True(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">
	<!-- Multline
	Comment -->
	<svg></svg>`)).IsSvgImage())
	assert.False(t, DetectContentType([]byte{}).IsSvgImage())
	assert.False(t, DetectContentType([]byte("svg")).IsSvgImage())
	assert.False(t, DetectContentType([]byte("<svgfoo></svgfoo>")).IsSvgImage())
	assert.False(t, DetectContentType([]byte("text<svg></svg>")).IsSvgImage())
	assert.False(t, DetectContentType([]byte("<html><body><svg></svg></body></html>")).IsSvgImage())
	assert.False(t, DetectContentType([]byte(`<script>"<svg></svg>"</script>`)).IsSvgImage())
	assert.False(t, DetectContentType([]byte(`<!-- <svg></svg> inside comment -->
	<foo></foo>`)).IsSvgImage())
	assert.False(t, DetectContentType([]byte(`<?xml version="1.0" encoding="UTF-8"?>
	<!-- <svg></svg> inside comment -->
	<foo></foo>`)).IsSvgImage())
}

func TestIsPDF(t *testing.T) {
	pdf, _ := base64.StdEncoding.DecodeString("JVBERi0xLjYKJcOkw7zDtsOfCjIgMCBvYmoKPDwvTGVuZ3RoIDMgMCBSL0ZpbHRlci9GbGF0ZURlY29kZT4+CnN0cmVhbQp4nF3NPwsCMQwF8D2f4s2CNYk1baF0EHRwOwg4iJt/NsFb/PpevUE4Mjwe")
	assert.True(t, DetectContentType(pdf).IsPDF())
	assert.False(t, DetectContentType([]byte("plain text")).IsPDF())
}

func TestIsVideo(t *testing.T) {
	mp4, _ := base64.StdEncoding.DecodeString("AAAAGGZ0eXBtcDQyAAAAAGlzb21tcDQyAAEI721vb3YAAABsbXZoZAAAAADaBlwX2gZcFwAAA+gA")
	assert.True(t, DetectContentType(mp4).IsVideo())
	assert.False(t, DetectContentType([]byte("plain text")).IsVideo())
}

func TestIsAudio(t *testing.T) {
	mp3, _ := base64.StdEncoding.DecodeString("SUQzBAAAAAABAFRYWFgAAAASAAADbWFqb3JfYnJhbmQAbXA0MgBUWFhYAAAAEQAAA21pbm9yX3Zl")
	assert.True(t, DetectContentType(mp3).IsAudio())
	assert.False(t, DetectContentType([]byte("plain text")).IsAudio())
}

func TestDetectContentTypeFromReader(t *testing.T) {
	mp3, _ := base64.StdEncoding.DecodeString("SUQzBAAAAAABAFRYWFgAAAASAAADbWFqb3JfYnJhbmQAbXA0MgBUWFhYAAAAEQAAA21pbm9yX3Zl")
	st, err := DetectContentTypeFromReader(bytes.NewReader(mp3))
	assert.NoError(t, err)
	assert.True(t, st.IsAudio())
}
