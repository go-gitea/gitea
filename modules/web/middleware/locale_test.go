// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAcceptLanguage(t *testing.T) {
	// a normal header is parsed and its leading language preserved
	tags := parseAcceptLanguage("de-DE,de;q=0.9,en;q=0.8")
	assert.NotEmpty(t, tags)
	assert.Equal(t, "de-DE", tags[0].String())

	// an oversized "_"-separated header would drive ParseAcceptLanguage into its
	// quadratic-time path (the built-in guard only counts "-"); the length bound
	// keeps the input passed to the parser small so it cannot be used for a DoS.
	malicious := strings.Repeat("_aaaaaaaaa", 1<<16) // ~640 KiB, zero "-" characters
	assert.Greater(t, len(malicious), maxAcceptLanguageLen)
	tags = parseAcceptLanguage(malicious)
	// no panic / hang, and nothing meaningful is parsed out of the garbage
	assert.Empty(t, tags)
}
