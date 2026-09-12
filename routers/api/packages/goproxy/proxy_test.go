// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoProxyList(t *testing.T) {
	t.Run("default list", func(t *testing.T) {
		specs, err := parseGoProxyList("https://proxy.golang.org,direct")
		require.NoError(t, err)
		require.Len(t, specs, 2)
		assert.Equal(t, proxyKindURL, specs[0].kind)
		assert.Equal(t, "https://proxy.golang.org", specs[0].url)
		assert.False(t, specs[0].fallbackOnError)
		assert.Equal(t, proxyKindDirect, specs[1].kind)
		assert.False(t, specs[1].fallbackOnError)
	})

	t.Run("pipe fallback", func(t *testing.T) {
		specs, err := parseGoProxyList("https://first.example|https://second.example")
		require.NoError(t, err)
		require.Len(t, specs, 2)
		assert.True(t, specs[0].fallbackOnError)
		assert.False(t, specs[1].fallbackOnError)
	})

	t.Run("off stops parsing", func(t *testing.T) {
		specs, err := parseGoProxyList("https://first.example,off,https://ignored.example")
		require.NoError(t, err)
		require.Len(t, specs, 2)
		assert.Equal(t, proxyKindURL, specs[0].kind)
		assert.Equal(t, proxyKindOff, specs[1].kind)
	})

	t.Run("rejects invalid url", func(t *testing.T) {
		_, err := parseGoProxyList("ftp://example.com")
		assert.Error(t, err)
	})

	t.Run("rejects empty list", func(t *testing.T) {
		_, err := parseGoProxyList("")
		assert.Error(t, err)
	})
}
