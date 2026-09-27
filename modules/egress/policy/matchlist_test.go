// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net/netip"
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
		// a bare entry matches its host exactly, subdomains need a wildcard or leading dot
		{pattern: "example.com", host: "example.com", port: 80, want: true},
		{pattern: "example.com", host: "sub.example.com", port: 80},
		{pattern: "*.example.com", host: "sub.example.com", port: 80, want: true},
		{pattern: ".example.com", host: "sub.example.com", port: 80, want: true},
		{pattern: "*.example.com", host: "example.com", port: 80}, // apex never matches
		{pattern: "*.example.com", host: "notexample.com", port: 80},
		// matching is case-insensitive and tolerates spaces, a port suffix and a trailing dot
		{pattern: "example.com", host: " EXAMPLE.com.:8080 ", port: 80, want: true},
		// allow entries without a port cover the default ports 80 and 443
		{pattern: "example.com", host: "example.com", port: 443, want: true},
		{pattern: "example.com", host: "example.com", port: 8080},
		{pattern: "example.com:*", host: "example.com", port: 8080, want: true},
		{pattern: "example.com:8080", host: "example.com", port: 8080, want: true},
		{pattern: "example.com:8080", host: "example.com", port: 80},
		{pattern: "example.com:[80|443-445]", host: "example.com", port: 444, want: true},
		{pattern: "example.com:[80|443-445]", host: "example.com", port: 446},
	} {
		assert.Equalf(t, tc.want, NewAllowList(tc.pattern).MatchHostname(tc.host, tc.port), "pattern %q host %q port %d", tc.pattern, tc.host, tc.port)
	}

	assert.True(t, NewAllowList(" , ").IsEmpty(), "blank entries are skipped")
	assert.True(t, NewAllowList("").IsEmpty())
	assert.False(t, NewAllowList("example.com").IsEmpty())
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
		{pattern: "private", ip: "100.64.0.1", port: 80, want: true}, // CGNAT
		{pattern: "private", ip: "8.8.8.8", port: 80},
		{pattern: "private:22", ip: "fd00::1", port: 22, want: true},
		{pattern: "private:22", ip: "fd00::1", port: 80},
	} {
		addr := netip.AddrPortFrom(netip.MustParseAddr(tc.ip), tc.port)
		assert.Equalf(t, tc.want, NewAllowList(tc.pattern).MatchIPAddr(addr), "pattern %q ip %s port %d", tc.pattern, tc.ip, tc.port)
	}
}

func TestBlockListDefaultPorts(t *testing.T) {
	// deny entries without a port cover every port, allow entries default to 80/443
	assert.True(t, NewBlockList("example.com").MatchHostname("example.com", 22))
	assert.True(t, NewBlockList("10.0.0.0/8").MatchIPAddr(netip.AddrPortFrom(netip.MustParseAddr("10.0.0.5"), 12345)))

	blocked := NewBlockList("example.com:22")
	assert.True(t, blocked.MatchHostname("example.com", 22))
	assert.False(t, blocked.MatchHostname("example.com", 80))
}

func TestRejectedEntries(t *testing.T) {
	for _, tc := range []struct{ entry, wantErr string }{
		{entry: "*", wantErr: `catch-all host pattern "*" matches every host and is not allowed`},
		{entry: "0.0.0.0/0", wantErr: `catch-all CIDR "0.0.0.0/0" covers every address and is not allowed`},
		{entry: "::/0", wantErr: `catch-all CIDR "::/0" covers every address and is not allowed`},
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
		list := NewAllowList(tc.entry)
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
	list := NewAllowList("example.com, 10.0.0.5/8, example.org")
	assert.True(t, list.MatchHostname("example.com", 80))
	assert.True(t, list.MatchHostname("example.org", 443))
	require.Len(t, list.Rejected(), 1)
	assert.Contains(t, list.Rejected()[0], `use "10.0.0.0/8"`)
	assert.Empty(t, NewAllowList("example.com").Rejected())
}
