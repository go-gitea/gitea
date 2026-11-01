// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchHostname(t *testing.T) {
	for _, tc := range []struct {
		pattern, host string
		port          uint16
		want          bool
	}{
		// a bare entry matches the exact host only, a wildcard or leading dot matches subdomains only
		{pattern: "example.com", host: "example.com", port: 80, want: true},
		{pattern: "example.com", host: "sub.example.com", port: 80}, // subdomains need the wildcard
		{pattern: "example.com", host: "notexample.com", port: 80},
		{pattern: "*.example.com", host: "sub.example.com", port: 80, want: true},
		{pattern: ".example.com", host: "sub.example.com", port: 80, want: true},
		{pattern: "*.example.com", host: "example.com", port: 80}, // apex never matches
		{pattern: "*.example.com", host: "notexample.com", port: 80},
		{pattern: ".example.com", host: "notexample.com", port: 80},
		// matching is case-insensitive and tolerates spaces, a port suffix and a trailing dot
		{pattern: "example.com", host: " EXAMPLE.com.:8080 ", port: 80, want: true},
		// strict entries without a port cover the web ports 80 and 443
		{pattern: "example.com", host: "example.com", port: 443, want: true},
		{pattern: "example.com", host: "example.com", port: 8080},
		{pattern: "example.com:*", host: "example.com", port: 8080, want: true},
		{pattern: "example.com:8080", host: "example.com", port: 8080, want: true},
		{pattern: "example.com:8080", host: "example.com", port: 80},
		{pattern: "example.com:[80|443-445]", host: "example.com", port: 444, want: true},
		{pattern: "example.com:[80|443-445]", host: "example.com", port: 446},
	} {
		assert.Equalf(t, tc.want, NewAllowList(tc.pattern, Strict).MatchHostname(tc.host, tc.port), "pattern %q host %q port %d", tc.pattern, tc.host, tc.port)
	}
	// lax entries without a port cover every port
	assert.True(t, NewAllowList("example.com", Lax).MatchHostname("example.com", 8080))

	assert.True(t, NewAllowList(" , ", Strict).IsEmpty(), "blank entries are skipped")
	assert.True(t, NewAllowList("", Strict).IsEmpty())
	assert.False(t, NewAllowList("example.com", Strict).IsEmpty())
}

func TestMatchIPAddr(t *testing.T) {
	for _, tc := range []struct {
		pattern, ip string
		port        uint16
		want        bool
	}{
		{pattern: "10.0.0.0/8", ip: "10.1.2.3", port: 80, want: true},
		{pattern: "10.0.0.0/8", ip: "11.1.2.3", port: 80},
		// a bare IP entry carries the default ports like a hostname entry
		{pattern: "192.168.1.1", ip: "192.168.1.1", port: 443, want: true},
		{pattern: "192.168.1.1", ip: "192.168.1.1", port: 8080},
		{pattern: "10.0.0.0/8:22", ip: "10.1.2.3", port: 22, want: true},
		{pattern: "10.0.0.0/8:22", ip: "10.1.2.3", port: 80},
		{pattern: "2001:db8::/64", ip: "2001:db8::1", port: 443, want: true},
		{pattern: "2001:db8::/64", ip: "2001:db8::1", port: 8080},
		{pattern: "[2001:db8::/64]:9418", ip: "2001:db8::1", port: 9418, want: true},
		{pattern: "[2001:db8::/64]:9418", ip: "2001:db9::1", port: 9418},
		{pattern: "[::1]:8080", ip: "::1", port: 8080, want: true},
		{pattern: "[::1]:8080", ip: "::1", port: 80},
		// named ranges expand to CIDRs
		{pattern: "loopback", ip: "127.0.0.1", port: 80, want: true},
		{pattern: "loopback", ip: "::1", port: 80, want: true},
		{pattern: "loopback", ip: "10.1.2.3", port: 80},
		{pattern: "private", ip: "100.64.0.1", port: 80}, // CGNAT is its own alias
		{pattern: "cgnat", ip: "100.64.0.1", port: 80, want: true},
		{pattern: "cgnat", ip: "100.100.100.200", port: 80, want: true},
		{pattern: "cgnat", ip: "10.1.2.3", port: 80},
		{pattern: "::ffff:10.0.0.0/104", ip: "10.1.2.3", port: 80, want: true}, // IPv4-mapped CIDR normalized to 10.0.0.0/8
		{pattern: "private", ip: "8.8.8.8", port: 80},
		{pattern: "private:22", ip: "fd00::1", port: 22, want: true},
		{pattern: "private:22", ip: "fd00::1", port: 80},
	} {
		addr := netip.AddrPortFrom(netip.MustParseAddr(tc.ip), tc.port)
		assert.Equalf(t, tc.want, NewAllowList(tc.pattern, Strict).MatchIPAddr(addr), "pattern %q ip %s port %d", tc.pattern, tc.ip, tc.port)
	}
}

func TestBlockListDefaultPorts(t *testing.T) {
	// deny entries without a port cover every port, allow entries default to the web ports in strict mode
	assert.True(t, NewBlockList("example.com").MatchHostname("example.com", 22))
	assert.True(t, NewBlockList("10.0.0.0/8").MatchIPAddr(netip.AddrPortFrom(netip.MustParseAddr("10.0.0.5"), 12345)))

	blocked := NewBlockList("example.com:22")
	assert.True(t, blocked.MatchHostname("example.com", 22))
	assert.False(t, blocked.MatchHostname("example.com", 80))
}

func TestRejectedEntries(t *testing.T) {
	for _, tc := range []struct{ entry, wantErr string }{
		{entry: "*", wantErr: `catch-all host pattern "*" matches every host and is not allowed`},
		{entry: "external", wantErr: "EGRESS_MODE"},
		{entry: "0.0.0.0/0", wantErr: `invalid CIDR "0.0.0.0/0": catch-all CIDR covers every address and is not allowed`},
		{entry: "::/0", wantErr: `invalid CIDR "::/0": catch-all CIDR covers every address and is not allowed`},
		// the whole mapped block unmaps to the catch-all; a shorter 4-in-6 prefix is not in the block and stays IPv6
		{entry: "::ffff:0:0/96", wantErr: "catch-all"},
		{entry: "10.0.0.5/8", wantErr: `host bits must be zero, use "10.0.0.0/8"`},
		{entry: "fe80::/64:80", wantErr: "unbracketed IPv6 with port"},
		{entry: "[2001:db8::]/64", wantErr: `unexpected "/64" after bracketed target`},
		{entry: "fe80::1%eth0", wantErr: "address zone is not dialable"},
		{entry: "999.1.1.1", wantErr: `target "999.1.1.1" looks like an IP address but is not a valid one`},
		{entry: "sub.*.example.com", wantErr: "is not an IP, CIDR, named range, or valid hostname pattern"},
		{entry: "exämple.com", wantErr: "is not an IP, CIDR, named range, or valid hostname pattern"},
		{entry: "example.com:99999", wantErr: `invalid port "99999"`},
		{entry: "example.com:443-80", wantErr: `reversed port range "443-80"`},
		{entry: "example.com:[]", wantErr: `empty port set "[]"`},
		{entry: ":80", wantErr: "empty host: ':80'"},
		{entry: "example.com:", wantErr: "empty port: 'example.com:'"},
		{entry: "[::1", wantErr: "missing closing bracket"},
		{entry: "[::1]:", wantErr: "empty port"},
		{entry: "[]:80", wantErr: "empty host"},
	} {
		list := NewAllowList(tc.entry, Strict)
		require.Lenf(t, list.Rejected(), 1, "entry %q", tc.entry)
		assert.Containsf(t, list.Rejected()[0], tc.wantErr, "entry %q", tc.entry)
	}

	// the deny list rejects the same catch-alls, a rejected entry matches nothing
	for _, entry := range []string{"*", "0.0.0.0/0", "::/0"} {
		block := NewBlockList(entry)
		require.Lenf(t, block.Rejected(), 1, "block entry %q", entry)
		assert.Truef(t, block.IsEmpty(), "block entry %q", entry)
	}

	// a rejected entry drops only itself
	list := NewAllowList("example.com, 10.0.0.5/8, example.org", Strict)
	assert.True(t, list.MatchHostname("example.com", 80))
	assert.True(t, list.MatchHostname("example.org", 443))
	require.Len(t, list.Rejected(), 1)
	assert.Contains(t, list.Rejected()[0], `use "10.0.0.0/8"`)
	assert.Empty(t, NewAllowList("example.com", Strict).Rejected())
}

func TestParseInternalRanges(t *testing.T) {
	prefixes, rejected := ParseInternalRanges("10.0.0.0/8, private, loopback, cgnat, 192.168.1.1, 2001:db8::/32, ,")
	assert.Empty(t, rejected)
	contains := func(ip string) bool {
		return inRange(prefixes, netip.MustParseAddr(ip))
	}
	assert.True(t, contains("10.1.2.3"))
	assert.True(t, contains("172.16.0.1"))      // private
	assert.True(t, contains("192.168.0.1"))     // private and the single IP
	assert.True(t, contains("127.0.0.1"))       // loopback
	assert.True(t, contains("100.100.100.200")) // cgnat
	assert.True(t, contains("2001:db8::1"))
	assert.False(t, contains("8.8.8.8"))
	assert.False(t, contains("169.254.169.254"))

	// IPv4-mapped forms are normalized to the IPv4 form, like in the allow and block lists
	prefixes, rejected = ParseInternalRanges("::ffff:10.0.0.0/104, ::ffff:192.168.1.1")
	assert.Empty(t, rejected)
	assert.True(t, inRange(prefixes, netip.MustParseAddr("10.1.2.3")))     // ::ffff:10.0.0.0/104 -> 10.0.0.0/8
	assert.True(t, inRange(prefixes, netip.MustParseAddr("192.168.1.1")))  // ::ffff:192.168.1.1 -> 192.168.1.1/32
	assert.False(t, inRange(prefixes, netip.MustParseAddr("192.168.1.2"))) // the bare address is a /32

	// invalid entries are rejected and match nothing
	prefixes, rejected = ParseInternalRanges("example.com, *.example.com, 0.0.0.0/0, 10.0.0.5/8, fe80::1%eth0, 999.1.1.1")
	assert.Len(t, rejected, 6)
	assert.Empty(t, prefixes)

	// ports are not supported, a range covers every port
	_, rejected = ParseInternalRanges("10.0.0.0/8:80, [2001:db8::/64]:9418, private:22, 2001:db8::/64:9418")
	assert.Len(t, rejected, 4)
	for _, reason := range rejected {
		assert.Contains(t, reason, "ports are not supported")
	}

	// a mapped form that unmaps to the catch-all is rejected
	_, rejected = ParseInternalRanges("::ffff:0:0/96")
	assert.Len(t, rejected, 1)
	assert.Contains(t, rejected[0], "catch-all")

	prefixes, rejected = ParseInternalRanges("")
	assert.Empty(t, prefixes)
	assert.Empty(t, rejected)
}

// MatchHostnameInRanges behavior is covered by the ranges rows in TestCheckAddr (policy_test.go).

func TestNeverMatching(t *testing.T) {
	list := NewAllowList("::ffff:10.0.0.0/104, 224.0.0.0/4, 2001::/32, 10.0.0.0/8, example.com, 192.168.1.1", Strict)
	never := list.NeverMatching()
	assert.Len(t, never, 2) // the IPv4-mapped CIDR is normalized to 10.0.0.0/8 at parse time and matches
	joined := strings.Join(never, "\n")
	assert.Contains(t, joined, "224.0.0.0/4")
	assert.Contains(t, joined, "2001::/32")
	assert.True(t, list.MatchIPAddr(netip.AddrPortFrom(netip.MustParseAddr("10.1.2.3"), 80)), "::ffff:10.0.0.0/104 must block/match 10.1.2.3")

	assert.Empty(t, NewAllowList("10.0.0.0/8, example.com, 192.168.1.1", Strict).NeverMatching())
	assert.Len(t, NewBlockList("224.0.0.0/4").NeverMatching(), 1)

	// partially reserved prefixes still match dialable addresses and are not flagged
	assert.Empty(t, NewAllowList("0.0.0.0/7, 2001::/16, 10.0.0.0/8", Strict).NeverMatching())
	// fully reserved prefixes are flagged, including subnets of a reserved range
	assert.Len(t, NewAllowList("0.0.0.0/8, 224.0.0.0/5, 2001::/32, 64:ff9b:1::/48", Strict).NeverMatching(), 4)
	// loopback is exempt from the reserved pool in classifyAddr, so the loopback alias
	// (127.0.0.0/8, ::1/128) is dialable and must not be flagged
	assert.Empty(t, NewAllowList("loopback", Strict).NeverMatching())
	assert.Empty(t, NewAllowList("127.0.0.0/8, ::1/128, ::/96", Strict).NeverMatching())
}
