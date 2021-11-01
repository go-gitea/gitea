// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

	hl := ParseHostMatchList("private, External, *.myDomain.com, 169.254.1.0/24")
	cases := []tc{
		{"", net.IPv4zero, false},
		{"", net.IPv6zero, false},

		{"", net.ParseIP("127.0.0.1"), false},
		{"", net.ParseIP("::1"), false},

		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"", net.ParseIP("fd00::1"), true},

		{"", net.ParseIP("8.8.8.8"), true},
		{"", net.ParseIP("1001::1"), true},

		{"mydomain.com", net.IPv4zero, false},
		{"sub.mydomain.com", net.IPv4zero, true},

		{"", net.ParseIP("169.254.1.1"), true},
		{"", net.ParseIP("169.254.2.2"), false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, hl.MatchesHostOrIP(c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	hl = ParseHostMatchList("loopback")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), true},
		{"", net.ParseIP("10.0.1.1"), false},
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("8.8.8.8"), false},

		{"", net.ParseIP("::1"), true},
		{"", net.ParseIP("fd00::1"), false},
		{"", net.ParseIP("1000::1"), false},

		{"mydomain.com", net.IPv4zero, false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, hl.MatchesHostOrIP(c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	hl = ParseHostMatchList("private")
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
	for _, c := range cases {
		assert.Equalf(t, c.expected, hl.MatchesHostOrIP(c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	hl = ParseHostMatchList("external")
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
	for _, c := range cases {
		assert.Equalf(t, c.expected, hl.MatchesHostOrIP(c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	hl = ParseHostMatchList("*")
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
	for _, c := range cases {
		assert.Equalf(t, c.expected, hl.MatchesHostOrIP(c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}
}
