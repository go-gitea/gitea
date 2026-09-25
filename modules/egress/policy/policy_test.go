// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

// Base-policy tests: the decision is the hostmatcher one (deny list rejects, then a non-empty
// allow list must match), plus the scheme-defaulted proxy exemption and the loopback proxy guard.

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPolicy(allow, block string, proxy *url.URL) *Policy {
	return NewPolicy("test",
		WithAllow(allow, "test.ALLOWED"),
		WithBlock(block, "test.BLOCKED"),
		WithProxy(proxy, nil))
}

func mustAddrPort(t *testing.T, s string) netip.AddrPort {
	t.Helper()
	ap, err := netip.ParseAddrPort(s)
	require.NoError(t, err, "bad addr %q", s)
	return ap
}

// dialPolicy returns the error from a policy-enforced dial, closing the connection on success.
func dialPolicy(t *testing.T, pol *Policy, addr string) error {
	t.Helper()
	conn, err := pol.NewDialContext()(context.Background(), "tcp", addr)
	if err == nil {
		_ = conn.Close()
	}
	return err
}

// TestCheckTarget pins the hostmatcher decision: the deny list rejects, then a non-empty allow
// list must match the host or the resolved IP; an empty allow list allows.
func TestCheckTarget(t *testing.T) {
	for _, tc := range []struct {
		name         string
		allow, block string
		host, ip     string
		want         bool
	}{
		{name: "empty lists allow", host: "github.com", ip: "8.8.8.8:80", want: true},
		{name: "empty lists allow private", ip: "10.0.0.5:80", want: true},
		{name: "block host", block: "evil.example.com", host: "evil.example.com", ip: "8.8.8.8:80", want: false},
		{name: "block cidr", block: "127.0.0.0/8", ip: "127.0.0.1:80", want: false},
		{name: "allow loopback only", allow: "loopback", ip: "127.0.0.1:80", want: true},
		{name: "allow list is a whitelist", allow: "loopback", ip: "8.8.8.8:80", want: false},
		{name: "hostname allow", allow: "example.com", host: "example.com", ip: "8.8.8.8:80", want: true},
		{name: "hostname allow does not grant others", allow: "example.com", host: "other.com", ip: "8.8.8.8:80", want: false},
		{name: "cidr allow", allow: "10.0.0.0/8", ip: "10.0.0.5:80", want: true},
		{name: "cidr allow does not grant others", allow: "10.0.0.0/8", ip: "192.168.0.1:80", want: false},
		{name: "block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5:80", want: false},
		{name: "block overrides hostname allow", allow: "*.example.com", block: "evil.example.com", host: "evil.example.com", ip: "8.8.8.8:80", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := testPolicy(tc.allow, tc.block, nil).checkTarget(tc.host, mustAddrPort(t, tc.ip))
			assert.Equal(t, tc.want, err == nil, "err = %v", err)
		})
	}
}

// TestDenialNamesSetting pins the hint: a rejection names the setting the offending list was read
// from, and the pointer is omitted when the policy was built without a key.
func TestDenialNamesSetting(t *testing.T) {
	denied := NewPolicy("webhook", WithAllow("example.com", "security.ALLOWED_HOST_LIST"))
	err := denied.checkTarget("other.com", mustAddrPort(t, "8.8.8.8:80"))
	require.Error(t, err)
	assert.Equal(t,
		"webhook can only call allowed HTTP servers (check your security.ALLOWED_HOST_LIST setting), deny 'other.com(8.8.8.8:80)'",
		err.Error())

	blocked := NewPolicy("webhook", WithBlock("loopback", "security.BLOCKED_HOST_LIST"))
	err = blocked.checkTarget("localhost", mustAddrPort(t, "127.0.0.1:80"))
	require.Error(t, err)
	assert.Equal(t,
		"webhook can not call blocked HTTP servers (check your security.BLOCKED_HOST_LIST setting), deny 'localhost(127.0.0.1:80)'",
		err.Error())

	bare := NewPolicy("test", WithAllow("example.com", ""))
	err = bare.checkTarget("other.com", mustAddrPort(t, "8.8.8.8:80"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "check your", "a policy built without a setting key must not invent one")
}

// TestCheckHostIPs pins the hostmatcher pre-check: the deny list rejects, and a non-empty allow
// list requires the host or every resolved address to match.
func TestCheckHostIPs(t *testing.T) {
	pol := testPolicy("", "blocked.example.com", nil)
	assert.NoError(t, pol.CheckHostIPs("example.com", []net.IP{net.ParseIP("8.8.8.8")}))
	assert.NoError(t, pol.CheckHostIPs("example.com", []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.5")}))
	assert.Error(t, pol.CheckHostIPs("blocked.example.com", []net.IP{net.ParseIP("8.8.8.8")}))
	assert.NoError(t, pol.CheckHostIPs("example.com", nil))
	assert.Error(t, pol.CheckHostIPs("blocked.example.com", nil))

	granted := testPolicy("10.0.0.0/8", "", nil)
	assert.NoError(t, granted.CheckHostIPs("example.com", []net.IP{net.ParseIP("10.0.0.5")}))
	assert.Error(t, granted.CheckHostIPs("example.com", []net.IP{net.ParseIP("192.168.0.1")}))
	assert.Error(t, granted.CheckHostIPs("example.com", []net.IP{net.ParseIP("10.0.0.5"), net.ParseIP("192.168.0.1")}))
	assert.NoError(t, granted.CheckHostIPs("", []net.IP{net.ParseIP("10.0.0.5")}))

	byHost := testPolicy("*.example.com", "", nil)
	assert.NoError(t, byHost.CheckHostIPs("git.example.com", []net.IP{net.ParseIP("192.168.0.1")}))
	assert.Error(t, byHost.CheckHostIPs("evil.example.net", []net.IP{net.ParseIP("192.168.0.1")}))
}

// TestDialContextDeniesAsErrDenied pins the dial path's error contract: a policy denial is
// wrapped in ErrDenied, so callers can map it apart from an ordinary network failure.
func TestDialContextDeniesAsErrDenied(t *testing.T) {
	err := dialPolicy(t, testPolicy("", "loopback", nil), "127.0.0.1:80")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDenied)
}

// TestDialContextProxyExemption pins the one improvement over hostmatcher.NewDialContext: the
// exemption compares the scheme-defaulted proxy port, so a portless PROXY_URL still exempts
// the address the transport actually dials.
func TestDialContextProxyExemption(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	addr := ln.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	// loopback is blocked, so only the proxy exemption can let the dial through
	const block = "loopback"

	t.Run("explicit proxy port", func(t *testing.T) {
		u, err := url.Parse("http://127.0.0.1:" + port)
		require.NoError(t, err)
		assert.NoError(t, dialPolicy(t, testPolicy("", block, u), addr))
	})

	t.Run("different port is not exempt", func(t *testing.T) {
		u, err := url.Parse("http://127.0.0.1:1")
		require.NoError(t, err)
		assert.ErrorIs(t, dialPolicy(t, testPolicy("", block, u), addr), ErrDenied)
	})

	t.Run("portless proxy defaults from the scheme", func(t *testing.T) {
		u, err := url.Parse("http://127.0.0.1")
		require.NoError(t, err)
		// 127.0.0.1:80 is the http scheme default; whatever the TCP outcome, the policy
		// must not deny it (hostmatcher's raw Port()=="" comparison never exempts it).
		err = dialPolicy(t, testPolicy("", block, u), "127.0.0.1:80")
		assert.ErrorIs(t, err, ErrDenied)
	})
}

func TestSchemePortAndProxyDialAddr(t *testing.T) {
	for scheme, want := range map[string]string{
		"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080", "ftp": "",
	} {
		assert.Equal(t, want, schemePort(scheme), scheme)
	}

	assert.Empty(t, ProxyDialAddr(nil))
	for raw, want := range map[string]string{
		"http://127.0.0.1":      "127.0.0.1:80",
		"https://127.0.0.1":     "127.0.0.1:443",
		"socks5://127.0.0.1":    "127.0.0.1:1080",
		"http://127.0.0.1:8080": "127.0.0.1:8080",
	} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, want, ProxyDialAddr(u), raw)
	}
}

func TestRestrictedRange(t *testing.T) {
	p := NewPolicy("test", WithAllow("external", "dummy"))
	// reserved ranges that IsPrivate does not cover: not external, but blockable as private
	for _, ip := range []string{
		"100.64.0.1",         // CGNAT
		"100.127.255.254",    // CGNAT
		"168.63.129.16",      // Azure WireServer
		"192.0.2.1",          // TEST-NET-1
		"198.18.0.1",         // benchmarking
		"198.51.100.1",       // TEST-NET-2
		"203.0.113.1",        // TEST-NET-3
		"169.254.169.254",    // Cloud metadata
		"192.88.99.1",        // 6to4 relay anycast
		"64:ff9b::1",         // NAT64
		"64:ff9b::a9fe:a9fe", // NAT64 embedding 169.254.169.254
		"2001::1",            // Teredo
		"2002::1",            // 6to4
		"2001:db8::1",        // documentation
		"fe80::1",            // link local address
	} {
		addr := netip.MustParseAddr(ip)
		addrPort := netip.AddrPortFrom(addr, 80)

		assert.Error(t, p.checkTarget("", addrPort), "reserved ip %s must not be external", ip)
		assert.Error(t, p.checkTarget("", addrPort), "reserved ip %s should match private block-list", ip)
	}
}
