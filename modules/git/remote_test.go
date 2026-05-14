// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSSHRemoteAddr(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		// SSH URL form
		{"ssh://git@github.com/go-gitea/gitea.git", true},
		{"ssh://git@host:22/path", true},
		{"git+ssh://git@github.com/go-gitea/gitea.git", true},
		{"  ssh://git@github.com/go-gitea/gitea.git  ", true}, // trims whitespace

		// SCP-like short form
		{"git@github.com:go-gitea/gitea.git", true},
		{"user@host:path/to/repo", true},
		{"git@host:/absolute/path/repo.git", true},

		// Not SSH
		{"https://github.com/go-gitea/gitea.git", false},
		{"http://github.com/go-gitea/gitea.git", false},
		{"git://github.com/go-gitea/gitea.git", false},
		{"/local/path/repo.git", false},
		{"file:///tmp/repo.git", false},
		{"", false},

		// Edge cases that look SCP-ish but aren't
		{"host:path", false},             // no '@'
		{"@host:path", false},            // '@' at start, no user
		{"git@host", false},              // no ':'
		{"http://user@host/path", false}, // '/' before ':' (after scheme)
		{"user@host/path:branch", false}, // '/' before ':' — looks like a URL path, not SCP
		{":git@host:path", false},        // ':' before '@'
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			assert.Equal(t, c.expected, IsSSHRemoteAddr(c.input))
		})
	}
}
