// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostOrIPMatchesList(t *testing.T) {
	type tc struct {
		host     string
		ip       net.IP
		expected bool
	}

	// for IPv6: "::1" is loopback, "fd00::/8" is private

	hl := ParseHostMatchList("", "private, External, *.myDomain.com, 169.254.1.0/24")

	test := func(cases []tc) {
		for _, c := range cases {
			assert.Equalf(t, c.expected, hl.MatchHostOrIP(c.host, c.ip), "case domain=%s, ip=%v, expected=%v", c.host, c.ip, c.expected)
		}
	}

	cases := []tc{
		{"", net.IPv4zero, false},
		{"", net.IPv6zero, false},

		{"", net.ParseIP("127.0.0.1"), false},
		{"127.0.0.1", nil, false},
		{"", net.ParseIP("::1"), false},

		{"", net.ParseIP("10.0.1.1"), true},
		{"10.0.1.1", nil, true},
		{"10.0.1.1:8080", nil, true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"192.168.1.1", nil, true},
		{"", net.ParseIP("fd00::1"), true},
		{"fd00::1", nil, true},

		{"", net.ParseIP("8.8.8.8"), true},
		{"", net.ParseIP("1001::1"), true},

		{"mydomain.com", net.IPv4zero, false},
		{"sub.mydomain.com", net.IPv4zero, true},
		{"sub.mydomain.com:8080", net.IPv4zero, true},

		{"", net.ParseIP("169.254.1.1"), true},
		{"169.254.1.1", nil, true},
		{"", net.ParseIP("169.254.2.2"), false},
		{"169.254.2.2", nil, false},
	}
	test(cases)

	hl = ParseHostMatchList("", "loopback")
	cases = []tc{
		{"", net.IPv4zero, true}, // 0.0.0.0 reaches localhost
		{"", net.ParseIP("127.0.0.1"), true},
		{"", net.ParseIP("10.0.1.1"), false},
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("8.8.8.8"), false},

		{"", net.ParseIP("::1"), true},
		{"", net.ParseIP("fd00::1"), false},
		{"", net.ParseIP("1000::1"), false},

		{"mydomain.com", nil, false},
	}
	test(cases)

	hl = ParseHostMatchList("", "private")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), false},
		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"", net.ParseIP("8.8.8.8"), false},

		{"", net.ParseIP("::1"), false},
		{"", net.ParseIP("fd00::1"), true},
		{"", net.ParseIP("1000::1"), false},

		{"mydomain.com", net.IPv4zero, false},
	}
	test(cases)

	hl = ParseHostMatchList("", "external")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), false},
		{"", net.ParseIP("10.0.1.1"), false},
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("8.8.8.8"), true},

		{"", net.ParseIP("::1"), false},
		{"", net.ParseIP("fd00::1"), false},
		{"", net.ParseIP("1000::1"), true},

		{"mydomain.com", net.IPv4zero, false},
	}
	test(cases)

	hl = ParseHostMatchList("", "*")
	cases = []tc{
		{"", net.IPv4zero, true},
		{"", net.ParseIP("127.0.0.1"), true},
		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"", net.ParseIP("8.8.8.8"), true},

		{"", net.ParseIP("::1"), true},
		{"", net.ParseIP("fd00::1"), true},
		{"", net.ParseIP("1000::1"), true},

		{"mydomain.com", net.IPv4zero, true},
	}
	test(cases)

	// built-in network names can be escaped (warping the first char with `[]`) to be used as a real host name
	// this mechanism is reversed for internal usage only (maybe for some rare cases), it's not supposed to be used by end users
	// a real user should never use loopback/private/external as their host names
	hl = ParseHostMatchList("", "loopback, [p]rivate")
	cases = []tc{
		{"loopback", nil, false},
		{"", net.ParseIP("127.0.0.1"), true},
		{"private", nil, true},
		{"", net.ParseIP("192.168.1.1"), false},
	}
	test(cases)

	hl = ParseSimpleMatchList("", "loopback, *.domain.com")
	cases = []tc{
		{"loopback", nil, true},
		{"", net.ParseIP("127.0.0.1"), false},
		{"sub.domain.com", nil, true},
		{"other.com", nil, false},
		{"", net.ParseIP("1.1.1.1"), false},
	}
	test(cases)

	hl = ParseSimpleMatchList("", "external")
	cases = []tc{
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("1.1.1.1"), false},
		{"external", nil, true},
	}
	test(cases)

	hl = ParseSimpleMatchList("", "")
	cases = []tc{
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("1.1.1.1"), false},
		{"external", nil, false},
	}
	test(cases)
}

func TestReservedRanges(t *testing.T) {
	external := ParseHostMatchList("", "external")
	private := ParseHostMatchList("", "private")
	reserved := ParseHostMatchList("", "reserved")

	// legitimate public destinations: external, not private
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "2001:4860:4860::8888", "1000::1"} {
		addr := net.ParseIP(ip)
		assert.Truef(t, external.MatchIPAddr(addr), "public ip %s should be external", ip)
		assert.Falsef(t, private.MatchIPAddr(addr), "public ip %s should not be private", ip)
	}

	// Private range edges, including RFC 6598 shared address space.
	for _, ip := range []string{
		"10.0.0.0", "10.255.255.255", // 10.0.0.0/8
		"172.16.0.0", "172.31.255.255", // 172.16.0.0/12
		"192.168.0.0", "192.168.255.255", // 192.168.0.0/16
		"100.64.0.0", "100.127.255.255", // 100.64.0.0/10
		"fc00::", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", // fc00::/7
	} {
		addr := net.ParseIP(ip)
		assert.Falsef(t, external.MatchIPAddr(addr), "private ip %s must not be external", ip)
		assert.Truef(t, private.MatchIPAddr(addr), "private ip %s should match private block-list", ip)
	}

	// 172.32.0.0 is just outside 172.16.0.0/12: a public destination, not private
	if addr := net.ParseIP("172.32.0.0"); assert.NotNil(t, addr) {
		assert.True(t, external.MatchIPAddr(addr), "172.32.0.0 should be external")
		assert.False(t, private.MatchIPAddr(addr), "172.32.0.0 should not be private")
	}

	// special-purpose ranges: neither external nor private, blockable as reserved
	for _, ip := range []string{
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
		addr := net.ParseIP(ip)
		assert.Falsef(t, external.MatchIPAddr(addr), "reserved ip %s must not be external", ip)
		assert.Falsef(t, private.MatchIPAddr(addr), "reserved ip %s must not be private", ip)
		assert.Truef(t, reserved.MatchIPAddr(addr), "reserved ip %s should match reserved block-list", ip)
	}
}
