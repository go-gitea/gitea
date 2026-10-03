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
		s := "🌞a🌛"
		assert.Equal(t, s[:1], tryFixTruncatedString(s[:1]))
		assert.Equal(t, s[:2], tryFixTruncatedString(s[:2]))
		assert.Equal(t, s[:3], tryFixTruncatedString(s[:3]))
		assert.Equal(t, "🌞", tryFixTruncatedString(s[:4]))
		assert.Equal(t, "🌞a", tryFixTruncatedString(s[:5]))
		assert.Equal(t, "🌞a", tryFixTruncatedString(s[:6]))
		assert.Equal(t, "🌞a", tryFixTruncatedString(s[:7]))
		assert.Equal(t, "🌞a", tryFixTruncatedString(s[:8]))
		assert.Equal(t, "🌞a🌛", tryFixTruncatedString(s[:9]))

		s = "a🌞🌛"
		assert.Equal(t, "a", tryFixTruncatedString(s[:1]))
		assert.Equal(t, "a", tryFixTruncatedString(s[:2]))
		assert.Equal(t, "a", tryFixTruncatedString(s[:3]))
		assert.Equal(t, "a", tryFixTruncatedString(s[:4]))
		assert.Equal(t, "a🌞", tryFixTruncatedString(s[:5]))
		assert.Equal(t, "a🌞", tryFixTruncatedString(s[:6]))
	})

	t.Run("Non-UTF8", func(t *testing.T) {
		s := "\xff\xee\xff\xeeb\xff\xee\xff\xee"
		assert.Equal(t, "\xff", tryFixTruncatedString(s[:1]))
		assert.Equal(t, "\xff\xee", tryFixTruncatedString(s[:2]))
		assert.Equal(t, "\xff\xee\xff", tryFixTruncatedString(s[:3]))
		assert.Equal(t, "\xff\xee\xff\xee", tryFixTruncatedString(s[:4]))
		assert.Equal(t, "\xff\xee\xff\xeeb", tryFixTruncatedString(s[:5]))
		assert.Equal(t, "\xff\xee\xff\xeeb", tryFixTruncatedString(s[:6]))
		assert.Equal(t, "\xff\xee\xff\xeeb", tryFixTruncatedString(s[:7]))
		assert.Equal(t, "\xff\xee\xff\xeeb", tryFixTruncatedString(s[:8]))
		assert.Equal(t, "\xff\xee\xff\xeeb\xff\xee\xff\xee", tryFixTruncatedString(s[:9]))
	})
}
