// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestInternalAPITLSHost(t *testing.T) {
	cases := []struct {
		name           string
		protocol       setting.Scheme
		localURL       string
		wantSkip       bool
		wantServerName string
	}{
		{"unix socket", setting.HTTPUnix, "http://localhost:3000/", true, ""},
		{"localhost", setting.HTTP, "http://localhost:3000/", true, ""},
		{"loopback ipv4", setting.HTTPS, "https://127.0.0.1:3000/", true, ""},
		{"loopback ipv6", setting.HTTPS, "https://[::1]:3000/", true, ""},
		// a non-loopback host must be verified, and then the ServerName must be the dialed internal host
		{"remote host", setting.HTTPS, "https://gitea.internal:443/", false, "gitea.internal"},
		{"remote ip", setting.HTTPS, "https://10.0.0.5:3000/", false, "10.0.0.5"},
		// an unparseable LOCAL_ROOT_URL can only be a hard misconfiguration; fail closed to verification
		{"invalid url", setting.HTTPS, "://bad", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			skip, serverName := internalAPITLSHost(c.protocol, c.localURL)
			assert.Equal(t, c.wantSkip, skip)
			assert.Equal(t, c.wantServerName, serverName)
		})
	}
}
