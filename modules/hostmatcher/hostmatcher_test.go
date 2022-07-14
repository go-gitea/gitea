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
