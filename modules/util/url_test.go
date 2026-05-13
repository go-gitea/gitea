// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://avatars.githubusercontent.com/u/9919", "https://avatars.githubusercontent.com/u/9919"},
		{"https://u:p@host.example.com/p?sig=deadbeef#frag", "https://host.example.com/p"},
		{"http://example.com/%ZZ", "<unparseable url>"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, StripURL(c.in))
	}
}
