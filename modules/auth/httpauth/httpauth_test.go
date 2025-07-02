// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httpauth

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAuthorizationHeader(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		cases := []struct {
			headerValue string
			user, pass  string
			ok          bool
		}{
			{"", "", "", false},
			{"?", "", "", false},
			{"foo", "", "", false},
			{"Basic ?", "", "", false},
			{"Basic " + base64.StdEncoding.EncodeToString([]byte("foo")), "", "", false},
			{"Basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar")), "foo", "bar", true},
			{"basic " + base64.StdEncoding.EncodeToString([]byte("foo:bar")), "foo", "bar", true},
		}
		for _, c := range cases {
			user, pass, ok := ParseAuthorizationHeaderBasic(c.headerValue)
			assert.Equal(t, c.ok, ok, "header %q", c.headerValue)
			assert.Equal(t, c.user, user, "header %q", c.headerValue)
			assert.Equal(t, c.pass, pass, "header %q", c.headerValue)
		}
	})
	t.Run("BearerToken", func(t *testing.T) {
		cases := []struct {
			headerValue string
			expected    string
			ok          bool
		}{
			{"", "", false},
			{"?", "", false},
			{"any value", "", false},
			{"token value", "value", true},
			{"Token value", "value", true},
			{"bearer value", "value", true},
			{"Bearer value", "value", true},
			{"Bearer wrong value", "", false},
		}
		for _, c := range cases {
			token, ok := ParseAuthorizationHeaderBearerToken(c.headerValue)
			assert.Equal(t, c.ok, ok, "header %q", c.headerValue)
			assert.Equal(t, c.expected, token, "header %q", c.headerValue)
		}
	})
}
