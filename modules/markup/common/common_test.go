// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLSchemes(t *testing.T) {
	t.Run("NoCustomSchemes", func(t *testing.T) {
		InitLinkURLSchemes(nil)
		assert.True(t, GlobalVars().LinkifyRegex.MatchString("http://example.com"))
		assert.True(t, GlobalVars().LinkifyRegex.MatchString("https://example.com"))
		assert.False(t, GlobalVars().LinkifyRegex.MatchString("some-other://example.com"))

		assert.Equal(t, CheckLinkURLSchemeResult{AllowToLinkify: true}, CheckLinkURLScheme("foo/:"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("HTTP://example.com"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("https://example.com"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("some-other:foo"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("any-other:bar"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: false}, CheckLinkURLScheme("javascript:void"))
	})

	t.Run("WithCustomSchemes", func(t *testing.T) {
		InitLinkURLSchemes([]string{"Some-Other"})
		assert.True(t, GlobalVars().LinkifyRegex.MatchString("http://example.com"))
		assert.True(t, GlobalVars().LinkifyRegex.MatchString("https://example.com"))
		assert.True(t, GlobalVars().LinkifyRegex.MatchString("some-other://example.com"))

		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("http://example.com"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("HTTPS://example.com"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: true}, CheckLinkURLScheme("some-other:foo"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: false}, CheckLinkURLScheme("any-other:bar"))
		assert.Equal(t, CheckLinkURLSchemeResult{HasScheme: true, AllowToLinkify: false}, CheckLinkURLScheme("JavaScript:void"))
	})
}
