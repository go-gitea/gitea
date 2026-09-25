// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAddr(t *testing.T) {
	for _, tc := range []struct {
		name, allow, block, host, ip string
		localNeedsIPAllow, want      bool
	}{
		{name: "empty lists allow", host: "github.com", ip: "8.8.8.8", want: true},
		{name: "empty lists allow private", ip: "10.0.0.5", want: true},
		{name: "block host", block: "evil.example.com", host: "evil.example.com", ip: "8.8.8.8"},
		{name: "block cidr", block: "127.0.0.0/8", ip: "127.0.0.1"},
		{name: "block ipv4-mapped", block: "loopback", ip: "::ffff:127.0.0.1"},
		{name: "block nat64 by cidr", block: "10.0.0.0/8", ip: "64:ff9b::a00:1"},
		{name: "allow loopback only", allow: "loopback", ip: "127.0.0.1", want: true},
		{name: "allow list rejects others", allow: "loopback", ip: "8.8.8.8"},
		{name: "allow host", allow: "example.com", host: "example.com", ip: "8.8.8.8", want: true},
		{name: "allow host rejects others", allow: "example.com", host: "other.com", ip: "8.8.8.8"},
		{name: "allow cidr", allow: "10.0.0.0/8", ip: "10.0.0.5", want: true},
		{name: "block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5"},
		{name: "reserved denied by wildcard", allow: "*", ip: "169.254.169.254"},
		{name: "reserved allowed by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254", want: true},
		{name: "local gate ignores host", allow: "example.com", host: "example.com", ip: "10.0.0.5", localNeedsIPAllow: true},
		{name: "local gate ignores wildcard", allow: "*", ip: "127.0.0.1", localNeedsIPAllow: true},
		{name: "local gate accepts builtin", allow: "private", ip: "100.64.0.1", localNeedsIPAllow: true, want: true},
		{name: "local gate accepts cidr", allow: "external, 10.0.0.0/24", ip: "10.0.0.5", localNeedsIPAllow: true, want: true},
	} {
		opts := []Option{WithAllow(tc.allow, "test.ALLOWED"), WithBlock(tc.block, "test.BLOCKED")}
		if tc.localNeedsIPAllow {
			opts = append(opts, WithLocalNeedsIPAllow())
		}
		err := NewPolicy("test", opts...).checkAddr(tc.host, netip.MustParseAddr(tc.ip))
		assert.Equal(t, tc.want, err == nil, "%s: %v", tc.name, err)
	}
}

func TestDenialNamesSetting(t *testing.T) {
	err := NewPolicy("webhook", WithAllow("example.com", "security.ALLOWED_HOST_LIST")).checkAddr("other.com", netip.MustParseAddr("8.8.8.8"))
	assert.EqualError(t, err, "webhook can only call allowed HTTP servers (check your security.ALLOWED_HOST_LIST setting), deny 'other.com(8.8.8.8)'")

	err = NewPolicy("webhook", WithBlock("evil.com", "migrations.BLOCKED_HOST_LIST")).CheckHost("evil.com")
	assert.EqualError(t, err, "webhook can not call blocked HTTP servers (check your migrations.BLOCKED_HOST_LIST setting), deny 'evil.com'")
}

func TestCheckHostIPs(t *testing.T) {
	ips := func(addrs ...string) (ret []net.IP) {
		for _, addr := range addrs {
			ret = append(ret, net.ParseIP(addr))
		}
		return ret
	}

	blocked := NewPolicy("test", WithBlock("blocked.example.com", ""))
	assert.NoError(t, blocked.CheckHostIPs("example.com", ips("8.8.8.8", "10.0.0.5")))
	assert.NoError(t, blocked.CheckHostIPs("example.com", nil))
	assert.Error(t, blocked.CheckHostIPs("blocked.example.com", ips("8.8.8.8")))
	assert.Error(t, blocked.CheckHostIPs("blocked.example.com", nil))

	allowed := NewPolicy("test", WithAllow("10.0.0.0/8, *.example.com", ""))
	assert.NoError(t, allowed.CheckHostIPs("", ips("10.0.0.5")))
	assert.NoError(t, allowed.CheckHostIPs("git.example.com", ips("192.168.0.1")))
	assert.NoError(t, allowed.CheckHostIPs("git.example.com", nil))
	assert.Error(t, allowed.CheckHostIPs("other.com", ips("10.0.0.5", "192.168.0.1")))
	assert.Error(t, allowed.CheckHostIPs("other.com", nil))

	builtins := NewPolicy("test", WithAllow("external, private, loopback", ""))
	assert.NoError(t, builtins.CheckHostIPs("example.com", ips("8.8.8.8", "64:ff9b::808:808", "100.64.0.1", "::1")))
	for _, ip := range []string{
		"0.1.2.3", "100.100.100.200", "168.63.129.16", "169.254.169.254", "192.0.2.1", "192.88.99.1", "198.18.0.1",
		"198.51.100.1", "203.0.113.1", "::7f00:1", "::ffff:0:a00:5", "64:ff9b::a9fe:a9fe", "2001::1", "2001:db8::1",
		"2002::1", "fe80::1",
	} {
		assert.Error(t, builtins.CheckHostIPs("example.com", ips(ip)), ip)
	}
}

func TestNewDialContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	addr := ln.Addr().String()

	dial := func(proxy string, allowProxies bool) error {
		policy := NewPolicy("test", WithBlock("loopback", ""), WithProxy(http.ProxyURL(&url.URL{Scheme: "http", Host: proxy})))
		_, _ = policy.Proxy(&http.Request{})
		conn, err := policy.dialContext(allowProxies)(t.Context(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}
	assert.NoError(t, dial(addr, true))
	assert.ErrorIs(t, dial(addr, false), ErrDenied)
	assert.ErrorIs(t, dial("127.0.0.1:1", true), ErrDenied)
}

func TestProxy(t *testing.T) {
	for raw, want := range map[string]string{
		"http://127.0.0.1":        "127.0.0.1:80",
		"socks5://[::1]":          "[::1]:1080",
		"https://proxy.corp:8443": "proxy.corp:8443",
	} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, want, ProxyDialAddr(u), raw)
	}

	_, err := NewPolicy("test", WithProxy(http.ProxyURL(&url.URL{Scheme: "socks4", Host: "proxy.corp:1080"}))).Proxy(&http.Request{})
	assert.ErrorContains(t, err, "unsupported proxy scheme")
}
