// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeSSHURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSH-SCP format with user",
			input:    "git@github.com:user/repo.git",
			expected: "ssh://git@github.com/user/repo.git",
		},
		{
			name:     "SSH-SCP format without user",
			input:    "github.com:user/repo.git",
			expected: "ssh://git@github.com/user/repo.git",
		},
		{
			name:     "Already ssh:// format",
			input:    "ssh://git@github.com/user/repo.git",
			expected: "ssh://git@github.com/user/repo.git",
		},
		{
			name:     "HTTP URL unchanged",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "Custom SSH user",
			input:    "myuser@example.com:path/to/repo.git",
			expected: "ssh://myuser@example.com/path/to/repo.git",
		},
		{
			name:     "Complex path",
			input:    "git@gitlab.com:group/subgroup/project.git",
			expected: "ssh://git@gitlab.com/group/subgroup/project.git",
		},
		{
			name:     "SSH with Port",
			input:    "ssh://git@example.com:2222/user/repo.git",
			expected: "ssh://git@example.com:2222/user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeSSHURL(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseRemoteAddrSSH(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		authUser    string
		authPass    string
		expected    string
		shouldError bool
	}{
		{
			name:        "SSH-SCP format normalized",
			remoteAddr:  "git@github.com:user/repo.git",
			authUser:    "",
			authPass:    "",
			expected:    "ssh://git@github.com/user/repo.git",
			shouldError: false,
		},
		{
			name:        "SSH URL with auth should error",
			remoteAddr:  "git@github.com:user/repo.git",
			authUser:    "user",
			authPass:    "pass",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "HTTPS URL with auth",
			remoteAddr:  "https://github.com/user/repo.git",
			authUser:    "user",
			authPass:    "pass",
			expected:    "https://user:pass@github.com/user/repo.git",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRemoteAddr(tt.remoteAddr, tt.authUser, tt.authPass)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
