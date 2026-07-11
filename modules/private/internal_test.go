// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestInternalAPISkipTLSVerify(t *testing.T) {
	cases := []struct {
		name     string
		protocol setting.Scheme
		localURL string
		want     bool
	}{
		{"unix socket", setting.HTTPUnix, "http://localhost:3000/", true},
		{"localhost", setting.HTTP, "http://localhost:3000/", true},
		{"loopback ipv4", setting.HTTPS, "https://127.0.0.1:3000/", true},
		{"loopback ipv6", setting.HTTPS, "https://[::1]:3000/", true},
		{"remote host", setting.HTTPS, "https://gitea.internal:443/", false},
		{"remote ip", setting.HTTPS, "https://10.0.0.5:3000/", false},
		{"invalid url", setting.HTTPS, "://bad", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, internalAPISkipTLSVerify(c.protocol, c.localURL))
		})
	}
}

func TestInternalAPIServerName(t *testing.T) {
	defer test.MockVariableValue(&setting.Domain, "public.example.com")()

	// when verification is enabled the cert must match the dialed internal host, not the public domain
	assert.Equal(t, "gitea.internal", internalAPIServerName("https://gitea.internal:443/"))
	assert.Equal(t, "127.0.0.1", internalAPIServerName("https://127.0.0.1:3000/"))
	// an unparseable URL falls back to the public domain
	assert.Equal(t, "public.example.com", internalAPIServerName("://bad"))
}
