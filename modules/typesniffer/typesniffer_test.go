// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package typesniffer

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectContentTypeLongerThanSniffLen(t *testing.T) {
	// Longer than sniffLen detects something else.
	assert.NotEqual(t, "image/svg+xml", DetectContentType([]byte(`<!-- `+strings.Repeat("x", sniffLen)+` --><svg></svg>`)).contentType)
}

func TestIsTextFile(t *testing.T) {
	assert.True(t, DetectContentType([]byte{}).IsText())
	assert.True(t, DetectContentType([]byte("lorem ipsum")).IsText())
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
