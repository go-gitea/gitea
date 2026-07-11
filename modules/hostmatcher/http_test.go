// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProxyFunc(t *testing.T) {
	fixedProxy, err := url.Parse("http://proxy.example:3128")
	require.NoError(t, err)
	// simulate a configured proxy that would be used for every request
	base := func(*http.Request) (*url.URL, error) { return fixedProxy, nil }

	newReq := func(rawURL string) *http.Request {
		r, err := http.NewRequest(http.MethodGet, rawURL, nil)
		require.NoError(t, err)
		return r
	}

	t.Run("blocks non-allowed target on the proxy path", func(t *testing.T) {
		pf := NewProxyFunc("test", ParseHostMatchList("test", "example.com"), nil, base)
		_, err := pf(newReq("http://evil.example/"))
		assert.ErrorContains(t, err, "can only call allowed HTTP servers")
	})

	t.Run("blocks link-local metadata even with external allow-list", func(t *testing.T) {
		pf := NewProxyFunc("test", ParseHostMatchList("test", MatchBuiltinExternal), nil, base)
		_, err := pf(newReq("http://169.254.169.254/latest/meta-data/"))
		assert.ErrorContains(t, err, "can only call allowed HTTP servers")
	})

	t.Run("allows a listed target on the proxy path", func(t *testing.T) {
		pf := NewProxyFunc("test", ParseHostMatchList("test", "example.com"), nil, base)
		proxyURL, err := pf(newReq("http://example.com/avatar.png"))
		assert.NoError(t, err)
		assert.Equal(t, fixedProxy, proxyURL)
	})

	// a bare hostname must be resolved so an IP-based builtin can authorize it; without resolution the
	// `loopback`/`external` builtins never match a DNS name and every proxied fetch would be refused.
	t.Run("resolves a hostname so an IP builtin can allow it", func(t *testing.T) {
		pf := NewProxyFunc("test", ParseHostMatchList("test", MatchBuiltinLoopback), nil, base)
		proxyURL, err := pf(newReq("http://localhost/avatar.png"))
		assert.NoError(t, err)
		assert.Equal(t, fixedProxy, proxyURL)
	})

	t.Run("resolves a hostname so an external builtin refuses a loopback name", func(t *testing.T) {
		pf := NewProxyFunc("test", ParseHostMatchList("test", MatchBuiltinExternal), nil, base)
		_, err := pf(newReq("http://localhost/avatar.png"))
		assert.ErrorContains(t, err, "can only call allowed HTTP servers")
	})

	// the block-list must also be enforced on the proxy path (migration keeps a block-list and often an
	// empty allow-list); otherwise a proxy could reach a blocked target unchecked.
	t.Run("enforces the block-list on the proxy path", func(t *testing.T) {
		pf := NewProxyFunc("test", nil, ParseHostMatchList("test", MatchBuiltinLoopback), base)
		_, err := pf(newReq("http://localhost/"))
		assert.ErrorContains(t, err, "can not call blocked HTTP servers")
	})

	t.Run("no proxy configured leaves validation to the dialer", func(t *testing.T) {
		noProxy := func(*http.Request) (*url.URL, error) { return nil, nil } //nolint:nilnil // mimics proxy.Proxy() selecting no proxy
		pf := NewProxyFunc("test", ParseHostMatchList("test", MatchBuiltinExternal), nil, noProxy)
		proxyURL, err := pf(newReq("http://169.254.169.254/"))
		assert.NoError(t, err)
		assert.Nil(t, proxyURL)
	})
}
