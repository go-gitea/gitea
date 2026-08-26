// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression test for https://github.com/go-gitea/gitea/issues/38903
//
// When LOCAL_ROOT_URL is set explicitly to "%(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/"
// (the historical Cheat Sheet suggestion) and HTTP_ADDR is left at its default of 0.0.0.0,
// LocalURL used to resolve to the literal, unroutable address "https://0.0.0.0:443/". The
// internal API client can never verify TLS against that host, so every internal request
// (including SSH key checks via `gitea serv`) failed with a TLS handshake error.
func TestLoadServerFrom_LocalRootURLZeroAddr(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`[server]
PROTOCOL = https
DOMAIN = sub.example.com
HTTP_PORT = 443
LOCAL_ROOT_URL = %(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/
`)
	assert.NoError(t, err)
	loadServerFrom(cfg)
	assert.Equal(t, "0.0.0.0", HTTPAddr)
	assert.Equal(t, "https://localhost:443/", LocalURL)
}

func TestLoadServerFrom_LocalRootURLExplicitHostUnaffected(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`[server]
PROTOCOL = https
DOMAIN = sub.example.com
HTTP_PORT = 443
LOCAL_ROOT_URL = https://sub.example.com:443/
`)
	assert.NoError(t, err)
	loadServerFrom(cfg)
	assert.Equal(t, "https://sub.example.com:443/", LocalURL)
}
