// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpauth

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAuthorizationHeader(t *testing.T) {
	type parsed = ParsedAuthorizationHeader
	type basic = BasicAuth
	type bearer = BearerToken
	cases := []struct {
		headerValue string
		expected    parsed
		ok          bool
	}{
		{"", parsed{}, false},
		{"?", parsed{}, false},
		{"foo", parsed{}, false},
		{"any value", parsed{}, false},

		{"Basic ?", parsed{}, false},
		{"Basic " + base64.StdEncoding.EncodeToString([]byte("foo")), parsed{}, false},
		{"Basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar")), parsed{BasicAuth: &basic{"foo", "bar"}}, true},
		{"basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar")), parsed{BasicAuth: &basic{"foo", "bar"}}, true},

		{"token value", parsed{BearerToken: &bearer{"value"}}, true},
		{"Token value", parsed{BearerToken: &bearer{"value"}}, true},
		{"bearer value", parsed{BearerToken: &bearer{"value"}}, true},
		{"Bearer value", parsed{BearerToken: &bearer{"value"}}, true},
		{"Bearer wrong value", parsed{}, false},
	}
	for _, c := range cases {
		ret, ok := ParseAuthorizationHeader(c.headerValue)
		assert.Equal(t, c.ok, ok, "header %q", c.headerValue)
		assert.Equal(t, c.expected, ret, "header %q", c.headerValue)
	}
}
