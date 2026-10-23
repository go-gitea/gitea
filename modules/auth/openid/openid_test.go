// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openid

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenIDDiscoveryBlocksInternalHost(t *testing.T) {
	var reached atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// RedirectURL performs server-side discovery of the identifier URL; a loopback URL
	// must be refused at dial time instead of reaching the internal server
	_, err := RedirectURL(srv.URL, "http://example.com/callback", "http://example.com/")
	require.Error(t, err)
	assert.False(t, reached.Load(), "OpenID discovery must not reach an internal/loopback host")
}
