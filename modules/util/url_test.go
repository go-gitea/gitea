// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeURL(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "strips userinfo from https URL",
			input:    "https://user:secret@host.example.com/owner/repo.git",
			expected: "https://host.example.com/owner/repo.git",
		},
		{
			name:     "https without credentials is unchanged",
			input:    "https://host.example.com/owner/repo.git",
			expected: "https://host.example.com/owner/repo.git",
		},
		{
			name:     "scp-like SSH short form is returned as-is",
			input:    "git@github.com:go-gitea/gitea.git",
			expected: "git@github.com:go-gitea/gitea.git",
		},
		{
			name:     "scp-like SSH short form with whitespace is trimmed",
			input:    "  git@github.com:go-gitea/gitea.git  ",
			expected: "git@github.com:go-gitea/gitea.git",
		},
		{
			name:     "ssh:// URL strips userinfo like other URLs",
			input:    "ssh://git@github.com/go-gitea/gitea.git",
			expected: "ssh://github.com/go-gitea/gitea.git",
		},
		{
			name:     "local path is unchanged",
			input:    "/tmp/repo.git",
			expected: "/tmp/repo.git",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := SanitizeURL(c.input)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.expected, got)
		})
	}
}
