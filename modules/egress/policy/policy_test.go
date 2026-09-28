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
		name, allow, block, host, ip    string
		localNeedsIPAllow, strict, want bool
	}{
		// lax mode exempts public and unresolved targets, restricted ones need an allow entry
		{name: "empty lists allow public", host: "github.com", ip: "8.8.8.8", want: true},
		{name: "empty lists deny private", ip: "10.0.0.5"},
		{name: "lax exempts public despite allow list", allow: "example.com", host: "other.com", ip: "8.8.8.8", want: true},
		{name: "lax exempts unresolved despite allow list", allow: "example.com", host: "other.com", want: true},
		{name: "block unresolved", block: "evil.example.com", host: "evil.example.com"},
		{name: "block host", block: "evil.example.com", host: "evil.example.com", ip: "8.8.8.8"},
		{name: "block cidr", block: "127.0.0.0/8", ip: "127.0.0.1"},
		{name: "block ipv4-mapped", block: "loopback", ip: "::ffff:127.0.0.1"},
		{name: "block nat64 by cidr", block: "10.0.0.0/8", ip: "64:ff9b::a00:1"},
		{name: "allow loopback", allow: "loopback", ip: "127.0.0.1", want: true},
		{name: "allow host", allow: "example.com", host: "example.com", ip: "8.8.8.8", want: true},
		{name: "allow cidr", allow: "10.0.0.0/8", ip: "10.0.0.5", want: true},
		{name: "block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5"},
		{name: "reserved denied by wildcard", allow: "*", ip: "169.254.169.254"},
		{name: "reserved denied by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254"},
		{name: "reserved denied ipv4-mapped", allow: "169.254.0.0/16", ip: "::ffff:169.254.169.254"},
		{name: "local gate ignores host", allow: "example.com", host: "example.com", ip: "10.0.0.5", localNeedsIPAllow: true},
		{name: "local gate ignores wildcard", allow: "*", ip: "127.0.0.1", localNeedsIPAllow: true},
		{name: "local gate accepts builtin", allow: "private", ip: "100.64.0.1", localNeedsIPAllow: true, want: true},
		{name: "local gate accepts cidr", allow: "external, 10.0.0.0/24", ip: "10.0.0.5", localNeedsIPAllow: true, want: true},
		// strict mode requires every target to match the allow list, an empty list denies all
		{name: "strict denies public with empty list", ip: "8.8.8.8", strict: true},
		{name: "strict denies unresolved with empty list", host: "example.com", strict: true},
		{name: "strict allows unresolved host", allow: "example.com", host: "example.com", strict: true, want: true},
		{name: "strict rejects unlisted public", allow: "loopback", ip: "8.8.8.8", strict: true},
		{name: "strict rejects unmatched host", allow: "example.com", host: "other.com", ip: "8.8.8.8", strict: true},
		{name: "strict allows matched host", allow: "example.com", host: "example.com", ip: "8.8.8.8", strict: true, want: true},
		{name: "strict block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5", strict: true},
		{name: "strict reserved denied by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254", strict: true},
		{name: "strict local gate ignores host", allow: "example.com", host: "example.com", ip: "10.0.0.5", localNeedsIPAllow: true, strict: true},
		{name: "strict local gate accepts builtin", allow: "private", ip: "100.64.0.1", localNeedsIPAllow: true, strict: true, want: true},
	} {
		opts := []Option{WithAllow(tc.allow, "test.ALLOWED"), WithBlock(tc.block, "test.BLOCKED")}
		if tc.localNeedsIPAllow {
			opts = append(opts, WithLocalNeedsIPAllow())
		}
		var addr netip.Addr
		if tc.ip != "" {
			addr = netip.MustParseAddr(tc.ip)
		}
		mode := Lax
		if tc.strict {
			mode = Strict
		}
		err := NewPolicy("test", mode, opts...).checkAddr(tc.host, netip.AddrPortFrom(addr, 80))
		assert.Equal(t, tc.want, err == nil, "%s: %v", tc.name, err)
	}
}

// a policy without list options behaves like one with empty lists
func TestPolicyWithoutLists(t *testing.T) {
	lax := NewPolicy("test", Lax)
	assert.NoError(t, lax.checkAddr("", netip.AddrPortFrom(netip.MustParseAddr("8.8.8.8"), 80))) // public targets pass
	assert.Error(t, lax.checkAddr("", netip.AddrPortFrom(netip.MustParseAddr("10.0.0.5"), 80)))  // restricted targets need an entry
	strict := NewPolicy("test", Strict)
	err := strict.checkAddr("", netip.AddrPortFrom(netip.MustParseAddr("8.8.8.8"), 80)) // strict denies without an entry
	assert.ErrorContains(t, err, "can only call allowed HTTP servers")
}

func TestDenialNamesSetting(t *testing.T) {
	err := NewPolicy("webhook", Strict, WithAllow("example.com", "security.ALLOWED_HOST_LIST")).checkAddr("other.com", netip.AddrPortFrom(netip.MustParseAddr("8.8.8.8"), 80))
	assert.EqualError(t, err, "webhook can only call allowed HTTP servers (check your security.ALLOWED_HOST_LIST setting), deny 'other.com(8.8.8.8)'")

	err = NewPolicy("webhook", Lax, WithBlock("evil.com", "migrations.BLOCKED_HOST_LIST")).CheckHost(hostURL(t, "http://evil.com"))
	assert.EqualError(t, err, "webhook can not call blocked HTTP servers (check your migrations.BLOCKED_HOST_LIST setting), deny 'evil.com'")
}

func hostURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestCheckHostIPs(t *testing.T) {
	ips := func(addrs ...string) (ret []net.IP) {
		for _, addr := range addrs {
			ret = append(ret, net.ParseIP(addr))
		}
		return ret
	}

	blocked := NewPolicy("test", Lax, WithAllow("private", ""), WithBlock("blocked.example.com", ""))
	assert.NoError(t, blocked.checkHostIPs(hostURL(t, "http://example.com"), ips("8.8.8.8", "10.0.0.5")))
	assert.NoError(t, blocked.checkHostIPs(hostURL(t, "http://example.com"), nil)) // unresolved name, the dialer re-checks the resolved address
	assert.Error(t, blocked.checkHostIPs(hostURL(t, "http://blocked.example.com"), ips("8.8.8.8")))
	assert.Error(t, blocked.checkHostIPs(hostURL(t, "http://blocked.example.com"), nil))

	allowed := NewPolicy("test", Strict, WithAllow("10.0.0.0/8, *.example.com", ""))
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://"), ips("10.0.0.5")))
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://git.example.com"), ips("192.168.0.1")))
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://git.example.com"), nil))
	assert.Error(t, allowed.checkHostIPs(hostURL(t, "http://other.com"), ips("10.0.0.5", "192.168.0.1")))
	assert.Error(t, allowed.checkHostIPs(hostURL(t, "http://other.com"), nil))

	builtins := NewPolicy("test", Lax, WithAllow("external, private, loopback", ""))
	assert.NoError(t, builtins.checkHostIPs(hostURL(t, "http://example.com"), ips("8.8.8.8", "100.64.0.1", "::1")))
	for _, ip := range []string{
		"0.1.2.3", "100.100.100.200", "168.63.129.16", "169.254.169.254", "192.0.2.1", "192.88.99.1", "198.18.0.1",
		"198.51.100.1", "203.0.113.1", "::7f00:1", "::ffff:0:a00:5", "64:ff9b::a9fe:a9fe", "64:ff9b::808:808", "2001::1", "2001:db8::1",
		"2002::1", "fe80::1",
	} {
		assert.Error(t, builtins.checkHostIPs(hostURL(t, "http://example.com"), ips(ip)), ip)
	}
}

func TestNewDialContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	addr := ln.Addr().String()

	dial := func(proxy string, allowProxies bool) error {
		policy := NewPolicy("test", Lax, WithBlock("loopback", ""), WithProxy(http.ProxyURL(&url.URL{Scheme: "http", Host: proxy})))
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

	_, err := NewPolicy("test", Lax, WithProxy(http.ProxyURL(&url.URL{Scheme: "socks4", Host: "proxy.corp:1080"}))).Proxy(&http.Request{})
	assert.ErrorContains(t, err, "unsupported proxy scheme")
}
