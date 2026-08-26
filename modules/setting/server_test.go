// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression test for https://github.com/go-gitea/gitea/issues/38903
//
// If LOCAL_ROOT_URL is explicitly set with the unspecified bind-all address as its host,
// LocalURL used to resolve to that literal, unroutable address, eg "https://0.0.0.0:443/".
// The internal API client can never verify TLS against that host, so every internal
// request (including SSH key checks via `gitea serv`) failed with a TLS handshake error.
func TestLoadServerFrom_LocalRootURLZeroAddr(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`[server]
PROTOCOL = https
DOMAIN = sub.example.com
HTTP_PORT = 443
LOCAL_ROOT_URL = https://0.0.0.0:443/
`)
	assert.NoError(t, err)
	loadServerFrom(cfg)
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
