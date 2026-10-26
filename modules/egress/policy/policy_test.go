// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
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
		name, allow, block, host, ip string
		strict, want                 bool
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
		{name: "non cloud link-local is default denied", ip: "::ffff:169.254.1.2"},
		{name: "link-local allowed by cidr", allow: "169.254.0.0/16", ip: "169.254.169.254", want: true},
		{name: "link-local allowed ipv4-mapped", allow: "169.254.0.0/16", ip: "::ffff:169.254.169.254", want: true},
		{name: "restricted range allowed by cidr", allow: "192.0.2.0/24", ip: "192.0.2.1", want: true},
		{name: "ula metadata allowed by private", allow: "private", ip: "fd00:ec2::254", want: true},
		{name: "reserved denied despite allow", allow: "168.63.129.16/32", ip: "168.63.129.16"},
		{name: "reserved denied ipv4-mapped", allow: "168.63.129.16/32", ip: "::ffff:168.63.129.16"},
		{name: "nat64 reserved denied despite allow", allow: "64:ff9b::/96", ip: "64:ff9b::a9fe:a9fe"},
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
	} {
		opts := []Option{WithAllow(tc.allow, "test.ALLOWED"), WithBlock(tc.block, "test.BLOCKED")}
		var addr netip.Addr
		if tc.ip != "" {
			addr = netip.MustParseAddr(tc.ip)
		}
		mode := Lax
		if tc.strict {
			mode = Strict
		}
		err := NewPolicy("test", mode, opts...).checkAddr(tc.host, netip.AddrPortFrom(addr, 80))
		assert.Equal(t, tc.want, err == nil, "%s (%s): %v", tc.name, tc.ip, err)
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
	assert.EqualError(t, err, "webhook can only call allowed HTTP servers (no allow entry matches, check your security.ALLOWED_HOST_LIST setting), deny 'other.com(8.8.8.8)'")

	err = NewPolicy("webhook", Lax, WithBlock("evil.com", "migrations.BLOCKED_HOST_LIST")).CheckHost(hostURL(t, "http://evil.com"))
	assert.EqualError(t, err, `webhook can not call blocked HTTP servers (matched hostname entry "evil.com", check your migrations.BLOCKED_HOST_LIST setting), deny 'evil.com'`)
}

func TestDenialReasons(t *testing.T) {
	addr := func(ip string, port uint16) netip.AddrPort {
		return netip.AddrPortFrom(netip.MustParseAddr(ip), port)
	}

	// reserved names the matching range, zoned addresses say so
	lax := NewPolicy("test", Lax)
	err := lax.checkAddr("", addr("64:ff9b::a00:1", 80))
	assert.EqualError(t, err, "test can not call reserved addresses (64:ff9b::/96), deny '64:ff9b::a00:1'")
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
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)' — set test.ALLOWED_INTERNAL_RANGES to let hostname entries cover non-public targets")
	err = restricted.checkAddr("other.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'other.com(10.1.2.3)'")

	// the same clear message in strict mode
	strict := NewPolicy("test", Strict, WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"), WithInternalRanges("", "test.ALLOWED_INTERNAL_RANGES"))
	err = strict.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, "test can not call non-public addresses without an allow entry (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)' — set test.ALLOWED_INTERNAL_RANGES to let hostname entries cover non-public targets")
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
		"198.51.100.1", "203.0.113.1", "100.100.100.200", "::7f00:1", "::ffff:0:a00:5", "64:ff9b::a9fe:a9fe", "64:ff9b::808:808", "2001::1", "2001:db8::1",
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

func TestInternalRanges(t *testing.T) {
	newPolicy := func(mode Mode, allow, ranges string) *Policy {
		return NewPolicy("test", mode,
			WithAllow(allow, "test.ALLOWED_HOST_LIST"),
			WithInternalRanges(ranges, "test.ALLOWED_INTERNAL_RANGES"))
	}
	addr := func(ip string, port uint16) netip.AddrPort {
		return netip.AddrPortFrom(netip.MustParseAddr(ip), port)
	}

	// a hostname entry covers the non-public targets it resolves into, the exact host only
	lax := newPolicy(Lax, "ci.internal.example.com", "10.0.0.0/8, 100.64.0.0/10")
	assert.NoError(t, lax.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80)))
	assert.NoError(t, lax.checkAddr("ci.internal.example.com", addr("100.100.100.200", 443)))
	assert.NoError(t, lax.checkAddr("ci.internal.example.com", addr("::ffff:10.1.2.3", 80))) // mapped form is unmapped, still via the domain
	assert.Error(t, lax.checkAddr("ci.internal.example.com", addr("192.168.5.5", 80)))       // restricted, outside the ranges
	assert.Error(t, lax.checkAddr("ci.internal.example.com", addr("169.254.169.254", 80)))
	assert.Error(t, lax.checkAddr("www.ci.internal.example.com", addr("10.1.2.3", 80))) // subdomain of a bare entry
	assert.Error(t, lax.checkAddr("other.com", addr("10.1.2.3", 80)))                   // unlisted host
	assert.Error(t, lax.checkAddr("", addr("10.1.2.3", 80)))                            // direct IP dial
	assert.Error(t, lax.checkAddr("", addr("::ffff:10.1.2.3", 80)))                     // direct IP dial, mapped form

	// a wildcard entry covers subdomains within the ranges, never the apex
	wildcard := newPolicy(Lax, "*.internal.example.com", "10.0.0.0/8")
	assert.NoError(t, wildcard.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80)))
	assert.Error(t, wildcard.checkAddr("internal.example.com", addr("10.1.2.3", 80)))       // apex
	assert.Error(t, wildcard.checkAddr("ci.internal.example.com", addr("192.168.5.5", 80))) // out of range

	// the denial names the ranges setting when the host matches but the IP is outside
	err := lax.checkAddr("ci.internal.example.com", addr("192.168.5.5", 80))
	assert.EqualError(t, err, "test resolves outside the internal ranges allowed for hostnames (check your test.ALLOWED_INTERNAL_RANGES setting), deny 'ci.internal.example.com(192.168.5.5)'")

	// a port-scoped entry covers only its ports
	scoped := newPolicy(Lax, "ci.internal.example.com:8080", "10.0.0.0/8")
	assert.NoError(t, scoped.checkAddr("ci.internal.example.com", addr("10.1.2.3", 8080)))
	err = scoped.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80))
	assert.EqualError(t, err, `test port 80 is not allowed for "ci.internal.example.com" (check your test.ALLOWED_HOST_LIST setting), deny 'ci.internal.example.com(10.1.2.3)'`)

	// without the ranges setting a hostname entry never covers non-public targets
	plain := newPolicy(Lax, "ci.internal.example.com", "")
	assert.Error(t, plain.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80)))

	// strict mode: the conjunction works, the domain's public IPs still match by name, portless entries cover 80/443 only
	strict := newPolicy(Strict, "ci.internal.example.com", "10.0.0.0/8")
	assert.NoError(t, strict.checkAddr("ci.internal.example.com", addr("10.1.2.3", 443)))
	assert.NoError(t, strict.checkAddr("ci.internal.example.com", addr("8.8.8.8", 443)))
	assert.Error(t, strict.checkAddr("ci.internal.example.com", addr("10.1.2.3", 8080)))
	assert.Error(t, strict.checkAddr("other.com", addr("10.1.2.3", 443)))

	// reserved is denied even inside the ranges
	reserved := newPolicy(Lax, "ci.internal.example.com", "224.0.0.0/4")
	assert.Error(t, reserved.checkAddr("ci.internal.example.com", addr("224.0.0.1", 80)))

	// the block list still wins
	blocked := NewPolicy("test", Lax,
		WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"),
		WithInternalRanges("10.0.0.0/8", "test.ALLOWED_INTERNAL_RANGES"),
		WithBlock("10.0.0.0/8", "test.BLOCKED_HOST_LIST"))
	assert.Error(t, blocked.checkAddr("ci.internal.example.com", addr("10.1.2.3", 80)))
}

func TestDialContextInternalRanges(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	policy := NewPolicy("test", Lax,
		WithAllow("localhost", "test.ALLOWED_HOST_LIST"),
		WithInternalRanges("loopback", "test.ALLOWED_INTERNAL_RANGES"))
	dial := policy.NewDialContext()

	conn, err := dial(t.Context(), "tcp", net.JoinHostPort("localhost", port))
	if err == nil {
		_ = conn.Close()
	}
	assert.NoError(t, err, "the exact host within the internal ranges is allowed")

	_, err = dial(t.Context(), "tcp", ln.Addr().String())
	assert.ErrorIs(t, err, ErrDenied, "a direct IP dial is denied even though the IP is in the ranges")
}

func TestCheckHostIPsInternalRanges(t *testing.T) {
	policy := NewPolicy("test", Lax,
		WithAllow("ci.internal.example.com", "test.ALLOWED_HOST_LIST"),
		WithInternalRanges("10.0.0.0/8", "test.ALLOWED_INTERNAL_RANGES"))
	ips := func(addrs ...string) (ret []net.IP) {
		for _, addr := range addrs {
			ret = append(ret, net.ParseIP(addr))
		}
		return ret
	}
	assert.NoError(t, policy.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("10.1.2.3")))
	assert.NoError(t, policy.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("10.1.2.3", "192.168.5.5"))) // one in-range address is enough
	assert.Error(t, policy.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("192.168.5.5", "172.16.0.1"))) // none in the ranges
	assert.Error(t, policy.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), ips("169.254.169.254")))
	assert.NoError(t, policy.checkHostIPs(hostURL(t, "http://ci.internal.example.com"), nil)) // unresolved, the dialer re-checks
}

func TestCheckHostIPsMultipleAddresses(t *testing.T) {
	ips := func(addrs ...string) (ret []net.IP) {
		for _, addr := range addrs {
			ret = append(ret, net.ParseIP(addr))
		}
		return ret
	}
	// the dialer tries the resolved addresses in turn, so one passing address is enough:
	// a DNS64-synthesized NAT64 address (reserved, never dialable) mixed with a regular IPv4
	lax := NewPolicy("test", Lax)
	assert.NoError(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::5db8:d822", "93.184.216.34")))
	assert.Error(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::5db8:d822")))             // only the NAT64 address
	assert.Error(t, lax.checkHostIPs(hostURL(t, "http://example.com"), ips("64:ff9b::5db8:d822", "10.0.0.5"))) // reserved + restricted, none passes
	// all addresses denied reports the first denial
	strict := NewPolicy("test", Strict, WithAllow("example.com", ""))
	err := strict.checkHostIPs(hostURL(t, "http://example.com"), ips("10.0.0.5", "192.168.0.1"))
	assert.ErrorContains(t, err, "10.0.0.5")
}
