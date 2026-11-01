// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAddr(t *testing.T) {
	for _, tc := range []struct {
		name, allow, block, ranges, host, ip string
		port                                 uint16
		strict, want                         bool
		wantErr                              string // substring of the expected denial error
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
		{name: "block nat64 by embedded ipv4 cidr", block: "10.0.0.0/8", ip: "64:ff9b::a00:1"},
		{name: "nat64 wkp decoded and allowed by embedded cidr", allow: "169.254.0.0/16", ip: "64:ff9b::a9fe:a9fe", want: true},
		{name: "allow loopback", allow: "loopback", ip: "127.0.0.1", want: true},
		{name: "allow host", allow: "example.com", host: "example.com", ip: "8.8.8.8", want: true},
		{name: "allow cidr", allow: "10.0.0.0/8", ip: "10.0.0.5", want: true},
		{name: "block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5"},
		{name: "non cloud link-local is default denied", ip: "::ffff:169.254.1.2"},
		{name: "link-local allowed by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254", want: true},
		{name: "link-local allowed ipv4-mapped", allow: "169.254.0.0/16", ip: "::ffff:169.254.169.254", want: true},
		{name: "restricted range allowed by cidr", allow: "192.0.2.0/24", ip: "192.0.2.1", want: true},
		{name: "ula metadata allowed by private", allow: "private", ip: "fd00:ec2::254", want: true},
		{name: "reserved denied despite allow", allow: "168.63.129.16/32", ip: "168.63.129.16"},
		{name: "reserved denied ipv4-mapped", allow: "168.63.129.16/32", ip: "::ffff:168.63.129.16"},
		{name: "nat64 wkp decoded, denied as restricted despite wkp allow", allow: "64:ff9b::/96", ip: "64:ff9b::a9fe:a9fe"},
		{name: "teredo reserved denied despite allow", allow: "2001::/23", ip: "2001::1"},
		{name: "protocol assignment allowed by cidr", allow: "2001::/23", ip: "2001:3::1", want: true},
		{name: "local gate ignores host", allow: "example.com", host: "example.com", ip: "10.0.0.5"},
		{name: "local gate accepts builtin", allow: "cgnat", ip: "100.64.0.1", want: true},
		{name: "private does not cover cgnat", allow: "private", ip: "100.64.0.1"},
		{name: "local gate accepts cidr", allow: "10.0.0.0/24", ip: "10.0.0.5", want: true},
		// strict mode requires every target to match the allow list, an empty list denies all
		{name: "strict denies public with empty list", ip: "8.8.8.8", strict: true},
		{name: "strict denies unresolved with empty list", host: "example.com", strict: true},
		{name: "strict allows unresolved host", allow: "example.com", host: "example.com", strict: true, want: true},
		{name: "strict rejects unlisted public", allow: "loopback", ip: "8.8.8.8", strict: true},
		{name: "strict rejects unmatched host", allow: "example.com", host: "other.com", ip: "8.8.8.8", strict: true},
		{name: "strict allows matched host", allow: "example.com", host: "example.com", ip: "8.8.8.8", strict: true, want: true},
		{name: "strict block overrides allow", allow: "10.0.0.0/8", block: "10.0.0.5/32", ip: "10.0.0.5", strict: true},
		{name: "strict reserved denied despite allow", allow: "168.63.129.16/32", ip: "168.63.129.16", strict: true},
		{name: "strict link-local allowed by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254", strict: true, want: true},
		{name: "strict local gate ignores host", allow: "example.com", host: "example.com", ip: "10.0.0.5", strict: true},
		{name: "strict local gate accepts builtin", allow: "cgnat", ip: "100.64.0.1", strict: true, want: true},
		// hostname entries cover non-public targets only within ALLOWED_INTERNAL_RANGES
		{name: "ranges allow host in range", allow: "ci.internal.example.com", ranges: "10.0.0.0/8, 100.64.0.0/10", host: "ci.internal.example.com", ip: "10.1.2.3", want: true},
		{name: "ranges allow cgnat in range", allow: "ci.internal.example.com", ranges: "10.0.0.0/8, 100.64.0.0/10", host: "ci.internal.example.com", ip: "100.100.100.200", port: 443, want: true},
		{name: "ranges allow mapped form", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "::ffff:10.1.2.3", want: true},
		{name: "ranges allow case and trailing dot", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "CI.INTERNAL.EXAMPLE.COM.", ip: "10.1.2.3", want: true},
		{name: "ranges deny outside", allow: "ci.internal.example.com", ranges: "10.0.0.0/8, 100.64.0.0/10", host: "ci.internal.example.com", ip: "192.168.5.5", wantErr: "resolves outside the internal ranges"},
		{name: "ranges deny link-local", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "169.254.169.254", wantErr: "resolves outside the internal ranges"},
		{name: "ranges allow subdomain of bare entry (v28 suffix matching)", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "www.ci.internal.example.com", ip: "10.1.2.3", want: true},
		{name: "ranges deny unlisted host", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "other.com", ip: "10.1.2.3", wantErr: "can not call non-public addresses without an allow entry"},
		{name: "ranges deny direct ip dial", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", ip: "10.1.2.3", wantErr: "can not call non-public addresses without an allow entry"},
		{name: "ranges deny direct mapped ip dial", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", ip: "::ffff:10.1.2.3", wantErr: "can not call non-public addresses without an allow entry"},
		{name: "ranges wildcard covers subdomains", allow: "*.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", want: true},
		{name: "ranges wildcard never covers apex", allow: "*.internal.example.com", ranges: "10.0.0.0/8", host: "internal.example.com", ip: "10.1.2.3"},
		{name: "ranges wildcard deny out of range", allow: "*.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "192.168.5.5", wantErr: "resolves outside the internal ranges"},
		{name: "ranges port-scoped allows its port", allow: "ci.internal.example.com:8080", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", port: 8080, want: true},
		{name: "ranges port-scoped denies other port", allow: "ci.internal.example.com:8080", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", wantErr: "port 80 is not allowed"},
		{name: "ranges portless lax covers every port", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", port: 8080, want: true},
		{name: "no ranges: hostname entry never covers non-public", allow: "ci.internal.example.com", host: "ci.internal.example.com", ip: "10.1.2.3"},
		{name: "strict ranges allow in range on 443", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", port: 443, strict: true, want: true},
		{name: "strict ranges allow public by name", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "8.8.8.8", port: 443, strict: true, want: true},
		{name: "strict ranges deny other port", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", port: 8080, strict: true},
		{name: "strict ranges deny unlisted host", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", host: "other.com", ip: "10.1.2.3", port: 443, strict: true},
		{name: "reserved denied even inside ranges", allow: "ci.internal.example.com", ranges: "224.0.0.0/4", host: "ci.internal.example.com", ip: "224.0.0.1", wantErr: "reserved"},
		{name: "block wins over ranges", allow: "ci.internal.example.com", ranges: "10.0.0.0/8", block: "10.0.0.0/8", host: "ci.internal.example.com", ip: "10.1.2.3", wantErr: "blocked"},
	} {
		opts := []Option{
			WithAllow(tc.allow, "test.ALLOWED"),
			WithBlock(tc.block, "test.BLOCKED"),
			WithInternalRanges(tc.ranges, "test.ALLOWED_INTERNAL_RANGES"),
		}
		var addr netip.Addr
		if tc.ip != "" {
			addr = netip.MustParseAddr(tc.ip)
		}
		mode := Lax
		if tc.strict {
			mode = Strict
		}
		port := tc.port
		if port == 0 {
			port = 80
		}
		err := NewPolicy("test", mode, opts...).checkAddr(tc.host, netip.AddrPortFrom(addr, port))
		if tc.wantErr != "" {
			assert.ErrorContains(t, err, tc.wantErr, "%s (%s)", tc.name, tc.ip)
		} else {
			assert.Equal(t, tc.want, err == nil, "%s (%s): %v", tc.name, tc.ip, err)
		}
	}
}

// a policy without list options behaves like one with empty lists
func addrPort(ip string, port uint16) netip.AddrPort {
	return netip.AddrPortFrom(netip.MustParseAddr(ip), port)
}

func TestDenialReasons(t *testing.T) {
	addr := addrPort

	// an allow list that matches nothing names the allow setting
	err := NewPolicy("webhook", Strict, WithAllow("example.com", "security.ALLOWED_HOST_LIST")).checkAddr("other.com", addrPort("8.8.8.8", 80))
	assert.EqualError(t, err, "webhook can only call allowed HTTP servers (no allow entry matches, check your security.ALLOWED_HOST_LIST setting), deny 'other.com(8.8.8.8)'")
	err = NewPolicy("webhook", Lax, WithBlock("evil.com", "migrations.BLOCKED_HOST_LIST")).CheckHost(hostURL(t, "http://evil.com"))
	assert.EqualError(t, err, `webhook can not call blocked HTTP servers (matched hostname entry "evil.com", check your migrations.BLOCKED_HOST_LIST setting), deny 'evil.com'`)

	// reserved names the matching range, zoned addresses say so
	lax := NewPolicy("test", Lax)
	err = lax.checkAddr("", addr("2001::1", 80))
	assert.EqualError(t, err, "test can not call reserved addresses (2001::/32), deny '2001::1'")
	// a NAT64 well-known-prefix address is decoded to the embedded IPv4 for scanning
	err = lax.checkAddr("", addr("64:ff9b::a9fe:a9fe", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your  setting), deny '169.254.169.254'")
	err = lax.checkAddr("", addr("fe80::1%eth0", 80))
	assert.EqualError(t, err, "test can not call reserved addresses (zoned address), deny 'fe80::1%eth0'")

	// a block names the matched entry, hostname or IP
	blocked := NewPolicy("test", Lax, WithBlock("evil.com, 10.0.0.0/8", "test.BLOCKED_HOST_LIST"))
	err = blocked.checkAddr("evil.com", addr("8.8.8.8", 80))
	assert.EqualError(t, err, `test can not call blocked HTTP servers (matched hostname entry "evil.com", check your test.BLOCKED_HOST_LIST setting), deny 'evil.com(8.8.8.8)'`)
	err = blocked.checkAddr("", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call blocked HTTP servers (matched IP entry 10.0.0.0/8, check your test.BLOCKED_HOST_LIST setting), deny '10.1.2.3'")

	// a restricted target names the allow setting, and hints at the internal ranges when a hostname entry matches
	restricted := NewPolicy("test", Lax, WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"), WithInternalRanges("", "test.ALLOWED_INTERNAL_RANGES"))
	err = restricted.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)' — set or fix test.ALLOWED_INTERNAL_RANGES to let hostname entries cover non-public targets")
	err = restricted.checkAddr("other.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'other.com(10.1.2.3)'")

	// the same clear message in strict mode
	strict := NewPolicy("test", Strict, WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"), WithInternalRanges("", "test.ALLOWED_INTERNAL_RANGES"))
	err = strict.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)' — set or fix test.ALLOWED_INTERNAL_RANGES to let hostname entries cover non-public targets")

	// set but nothing valid parsed (e.g. a typo): the hint says so instead of asking to set it again
	invalid := NewPolicy("test", Lax, WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"), WithInternalRanges("10.0.0.5/8", "test.ALLOWED_INTERNAL_RANGES"))
	err = invalid.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)' — set or fix test.ALLOWED_INTERNAL_RANGES to let hostname entries cover non-public targets")

	// a hostname entry with ranges gives the specific reason: out of range, or port not allowed
	ranged := NewPolicy("test", Lax, WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"), WithInternalRanges("10.0.0.0/8", "test.ALLOWED_INTERNAL_RANGES"))
	err = ranged.checkAddr("ci.internal.example.com", addr("192.168.5.5", 80))
	assert.EqualError(t, err, "test resolves outside the internal ranges allowed for hostnames (check your test.ALLOWED_INTERNAL_RANGES setting), deny 'ci.internal.example.com(192.168.5.5)'")
	scoped := NewPolicy("test", Lax, WithAllow("ci.internal.example.com:8080", "test.ALLOWED_HOST_LIST"), WithInternalRanges("10.0.0.0/8", "test.ALLOWED_INTERNAL_RANGES"))
	err = scoped.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, `test port 80 is not allowed for "ci.internal.example.com" (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)'`)
}

func hostURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func ips(addrs ...string) (ret []net.IP) {
	for _, addr := range addrs {
		ret = append(ret, net.ParseIP(addr))
	}
	return ret
}

func TestCheckHostIPs(t *testing.T) {
	blocked := NewPolicy("test", Lax, WithAllow("private", ""), WithBlock("blocked.example.com", ""))
	assert.NoError(t, blocked.checkHostIPs(hostURL(t, "http://example.com"), ips("8.8.8.8", "10.0.0.5")))
	assert.NoError(t, blocked.checkHostIPs(hostURL(t, "http://example.com"), nil)) // unresolved name, the dialer re-checks the resolved address
	assert.Error(t, blocked.checkHostIPs(hostURL(t, "http://blocked.example.com"), ips("8.8.8.8")))
	assert.Error(t, blocked.checkHostIPs(hostURL(t, "http://blocked.example.com"), nil))

	allowed := NewPolicy("test", Strict, WithAllow("10.0.0.0/8, *.example.com", ""))
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://"), ips("10.0.0.5")))
	assert.Error(t, allowed.checkHostIPs(hostURL(t, "http://git.example.com"), ips("192.168.0.1"))) // a hostname entry doesn't cover a restricted target
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://git.example.com"), ips("10.0.0.5")))  // an IP entry does
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://git.example.com"), nil))
	assert.NoError(t, allowed.checkHostIPs(hostURL(t, "http://other.com"), ips("10.0.0.5", "192.168.0.1"))) // one passing address is enough
	assert.Error(t, allowed.checkHostIPs(hostURL(t, "http://other.com"), ips("192.168.0.1", "172.16.0.1"))) // none passes
	assert.Error(t, allowed.checkHostIPs(hostURL(t, "http://other.com"), nil))

	builtins := NewPolicy("test", Lax, WithAllow("private, loopback", ""))
	assert.NoError(t, builtins.checkHostIPs(hostURL(t, "http://example.com"), ips("8.8.8.8", "10.0.0.5", "::1")))
	cgnat := NewPolicy("test", Lax, WithAllow("cgnat", ""))
	assert.NoError(t, cgnat.checkHostIPs(hostURL(t, "http://example.com"), ips("100.100.100.200"))) // cloud metadata is opt-in with its containing range
	for _, ip := range []string{
		"0.1.2.3", "168.63.129.16", "169.254.169.254", "192.0.2.1", "192.88.99.1", "198.18.0.1",
		"198.51.100.1", "203.0.113.1", "100.100.100.200", "::7f00:1", "::ffff:0:a00:5", "64:ff9b::a9fe:a9fe", "2001::1", "2001:db8::1",
		"2002::1", "fe80::1",
	} {
		assert.Error(t, builtins.checkHostIPs(hostURL(t, "http://example.com"), ips(ip)), ip)
	}

	t.Run("InternalRanges", func(t *testing.T) {
		p := NewPolicy("test", Lax,
			WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"),
			WithInternalRanges("10.0.0.0/8", "test.ALLOWED_INTERNAL_RANGES"))
		assert.NoError(t, p.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("10.1.2.3")))
		assert.NoError(t, p.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("10.1.2.3", "192.168.5.5"))) // one in-range address is enough
		assert.Error(t, p.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("192.168.5.5", "172.16.0.1"))) // none in the ranges
		assert.Error(t, p.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("169.254.169.254")))
		assert.NoError(t, p.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), nil)) // unresolved, the dialer re-checks
	})

	t.Run("MultipleAddresses", func(t *testing.T) {
		// the dialer tries the resolved addresses in turn, so one passing address is enough:
		// a DNS64-synthesized NAT64 address decodes to its embedded IPv4 (here 10.0.0.5, restricted)
		lax := NewPolicy("test", Lax)
		assert.NoError(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::a00:5", "93.184.216.34")))
		assert.Error(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::a00:5")))             // only the NAT64 address
		assert.Error(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::a00:5", "10.0.0.5"))) // decoded NAT64 + restricted, none passes
		// all addresses denied reports the first denial
		strict := NewPolicy("test", Strict, WithAllow("example.com", ""))
		err := strict.checkHostIPs(hostURL(t, "http://example.com"), ips("10.0.0.5", "192.168.0.1"))
		assert.ErrorContains(t, err, "10.0.0.5")
	})
}

func TestNewDialContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	addr := ln.Addr().String()

	dial := func(p *Policy) error {
		conn, err := p.NewDialContext()(t.Context(), "tcp", addr)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}
	// every dial is checked, a configured proxy's own address included
	assert.ErrorIs(t, dial(NewPolicy("test", Lax, WithBlock("loopback", ""))), ErrDenied)
	assert.ErrorIs(t, dial(NewPolicy("test", Lax, WithBlock("loopback", ""), WithProxy(http.ProxyURL(&url.URL{Scheme: "http", Host: addr})))), ErrDenied)
	assert.NoError(t, dial(NewPolicy("test", Lax, WithAllow("loopback", ""))))

	// a hostname within the internal ranges is allowed by name, a direct IP dial is not
	ranged := NewPolicy("test", Lax, WithAllow("localhost", "test.ALLOWED_HOST_LIST"), WithInternalRanges("loopback", "test.ALLOWED_INTERNAL_RANGES"))
	_, port, _ := net.SplitHostPort(addr)
	conn, err := ranged.NewDialContext()(t.Context(), "tcp", net.JoinHostPort("localhost", port))
	if err == nil {
		_ = conn.Close()
	}
	assert.NoError(t, err, "the exact host within the internal ranges is allowed")
	assert.ErrorIs(t, dial(ranged), ErrDenied, "a direct IP dial is denied even though the IP is in the ranges")
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

func TestDirectTargetAtProxyAddress(t *testing.T) {
	// the proxy must only ever see proxy-form requests, never an origin-form one
	proxy := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		assert.True(t, r.URL.IsAbs(), "the proxy must not be reached as an origin server")
	}))
	t.Cleanup(proxy.Close)
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	selectProxy := func(req *http.Request) (*url.URL, error) {
		if req.URL.Hostname() == "proxied.example.com" {
			return proxyURL, nil
		}
		return nil, nil //nolint:nilnil // no proxy for this request
	}
	// get primes the dialer's proxy exemption with one proxied request, then dials target directly
	get := func(p *Policy, target string) error {
		client := &http.Client{Transport: p.NewHTTPTransport(nil)}
		defer client.CloseIdleConnections()
		for _, u := range []string{"http://proxied.example.com/", target} {
			resp, err := client.Get(u)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
		}
		return nil
	}
	// a direct request to the proxy's own address is denied, in lax and strict mode,
	// and a block entry wins over an allow of the same address
	assert.ErrorIs(t, get(NewPolicy("test", Lax, WithProxy(selectProxy)), proxy.URL), ErrDenied)
	assert.ErrorIs(t, get(NewPolicy("test", Strict, WithAllow("proxied.example.com", ""), WithProxy(selectProxy)), proxy.URL), ErrDenied)
	assert.ErrorIs(t, get(NewPolicy("test", Lax, WithAllow("loopback", ""), WithBlock(proxyURL.Host, ""), WithProxy(selectProxy)), proxy.URL), ErrDenied)
	// a direct request to an allowed target still goes through
	origin := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(origin.Close)
	assert.NoError(t, get(NewPolicy("test", Lax, WithAllow("loopback", ""), WithProxy(selectProxy)), origin.URL))
}

func TestNewHTTPTransportConfiguresBase(t *testing.T) {
	// a base transport with its own Proxy selector and a DialTLSContext: the selector must be
	// captured for the proxyTransport and cleared on the direct transport (so it runs once per
	// request, not again inside the base transport), and DialTLSContext must be reset so TLS
	// dials go through the policy-checked DialContext instead of bypassing it
	origin := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(origin.Close)

	calls := 0
	base := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { calls++; return nil, nil },
		DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("DialTLSContext bypass")
		},
	}
	rt := NewPolicy("test", Lax, WithAllow("loopback", "test.ALLOWED_HOST_LIST")).NewHTTPTransport(base)
	pt, ok := rt.(*proxyTransport)
	require.True(t, ok)
	assert.Nil(t, base.Proxy, "the direct transport must not keep the selector")
	assert.Nil(t, base.DialTLSContext, "a custom TLS dialer would bypass the policy check")
	assert.NotNil(t, base.DialContext)
	assert.NotNil(t, pt.selector)

	req, err := http.NewRequest(http.MethodGet, origin.URL, nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, 1, calls, "the selector must run exactly once per request")
}

func TestProxiedTargetNeedsNoDNS(t *testing.T) {
	// a proxied request is not screened: the proxy resolves the target, so the request
	// must succeed even when the target's name does not resolve locally (.invalid never does)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, r.URL.IsAbs(), "the proxy must see the request in proxy form")
		_, _ = w.Write([]byte("proxied"))
	}))
	t.Cleanup(proxy.Close)
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	p := NewPolicy("test", Lax, WithProxy(func(req *http.Request) (*url.URL, error) {
		if req.URL.Hostname() == "no-such-host.invalid" {
			return proxyURL, nil
		}
		return nil, nil //nolint:nilnil // no proxy for this request
	}))
	client := &http.Client{Transport: p.NewHTTPTransport(nil)}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://no-such-host.invalid/repo.git")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "proxied", string(body))
}
