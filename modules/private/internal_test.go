// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestInternalAPIConnectionIsLocal(t *testing.T) {
	cases := []struct {
		name     string
		protocol setting.Scheme
		localURL string
		want     bool
	}{
		// HTTPUnix always dials the unix socket (a local target), whatever LOCAL_ROOT_URL says
		{"unix socket", setting.HTTPUnix, "https://gitea.example.com/", true},
		{"localhost", setting.HTTP, "http://localhost:3000/", true},
		{"loopback ipv4", setting.HTTPS, "https://127.0.0.1:3000/", true},
		{"loopback ipv6", setting.HTTPS, "https://[::1]:3000/", true},
		// a non-loopback LOCAL_ROOT_URL is a real network hop and must be verified
		{"remote host", setting.HTTPS, "https://gitea.internal:443/", false},
		{"remote ip", setting.HTTPS, "https://10.0.0.5:3000/", false},
		// an unparseable LOCAL_ROOT_URL is a hard misconfiguration; fail closed to verification
		{"invalid url", setting.HTTPS, "://bad", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, internalAPIConnectionIsLocal(c.protocol, c.localURL))
		})
	}
}
