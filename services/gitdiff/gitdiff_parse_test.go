// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTryFixTruncatedString(t *testing.T) {
	assert.Equal(t, "", tryFixTruncatedString(""))

	t.Run("UTF8", func(t *testing.T) {
		for _, test := range []struct {
			input    string
			expected []string
		}{
			{"🌞a🌛", []string{"🌞"[:1], "🌞"[:2], "🌞"[:3], "🌞", "🌞a", "🌞a", "🌞a", "🌞a", "🌞a🌛"}},
			{"a🌞🌛", []string{"a", "a", "a", "a", "a🌞", "a🌞"}},
		} {
			for i, expected := range test.expected {
				assert.Equal(t, expected, tryFixTruncatedString(test.input[:i+1]))
			}
		}
	})

	t.Run("Non-UTF8", func(t *testing.T) {
		s := "\xff\xee\xff\xeeb\xff\xee\xff\xee"
		for i, expected := range []string{
			"\xff", "\xff\xee", "\xff\xee\xff", "\xff\xee\xff\xee",
			"\xff\xee\xff\xeeb", "\xff\xee\xff\xeeb", "\xff\xee\xff\xeeb", "\xff\xee\xff\xeeb", "\xff\xee\xff\xeeb\xff\xee\xff\xee",
		} {
			assert.Equal(t, expected, tryFixTruncatedString(s[:i+1]))
		}
	})
}
