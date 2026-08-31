// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForwardedHeadersHandler(t *testing.T) {
	defaultProxies := []string{"127.0.0.0/8", "::1/128"}
	cases := []struct {
		name       string
		remoteAddr string
		limit      int
		proxies    []string
		header     http.Header
		expected   string
	}{
		{"rightmost entry", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1, 2.2.2.2"}}, "2.2.2.2:0"},
		{"two hops", "127.0.0.1:1234", 2, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1, 2.2.2.2"}}, "1.1.1.1:0"},
		{"chain shorter than limit", "127.0.0.1:1234", 3, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1, 2.2.2.2"}}, "1.1.1.1:0"},
		{"entries without space", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1,2.2.2.2"}}, "2.2.2.2:0"},
		{"repeated header", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1", "2.2.2.2, 3.3.3.3"}}, "3.3.3.3:0"},
		{"non-ip entry", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"evil, <script>"}}, "127.0.0.1:1234"},
		{"entry with port", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1:5678"}}, "127.0.0.1:1234"},
		{"mapped ipv4", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"::ffff:1.2.3.4"}}, "1.2.3.4:0"},
		{"ipv6 entry", "[::1]:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"2001:db8::1"}}, "[2001:db8::1]:0"},
		{"real ip wins", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Real-Ip": {"9.9.9.9"}, "X-Forwarded-For": {"1.1.1.1"}}, "9.9.9.9:0"},
		{"non-ip real ip", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Real-Ip": {"evil"}}, "127.0.0.1:1234"},
		{"empty real ip falls through", "127.0.0.1:1234", 1, defaultProxies, http.Header{"X-Real-Ip": {""}, "X-Forwarded-For": {"1.1.1.1"}}, "1.1.1.1:0"},
		{"untrusted peer", "8.8.8.8:1234", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1"}}, "8.8.8.8:1234"},
		{"bare ip proxy", "10.0.0.1:1234", 1, []string{"10.0.0.1"}, http.Header{"X-Forwarded-For": {"1.1.1.1"}}, "1.1.1.1:0"},
		{"wildcard proxy", "8.8.8.8:1234", 1, []string{"*"}, http.Header{"X-Forwarded-For": {"1.1.1.1"}}, "1.1.1.1:0"},
		{"unix socket", "@", 1, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1"}}, "1.1.1.1:0"},
		{"no header", "127.0.0.1:1234", 1, defaultProxies, http.Header{}, "127.0.0.1:1234"},
		{"zero limit", "127.0.0.1:1234", 0, defaultProxies, http.Header{"X-Forwarded-For": {"1.1.1.1, 2.2.2.2"}}, "2.2.2.2:0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			handler := ForwardedHeadersHandler(c.limit, c.proxies)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				got = req.RemoteAddr
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = c.remoteAddr
			req.Header = c.header
			handler.ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, c.expected, got)
		})
	}
}
