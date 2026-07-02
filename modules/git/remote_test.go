// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRemoteAddr(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		authUser    string
		authPass    string
		expected    string
		shouldError bool
	}{
		{
			name:       "SSH SCP short syntax normalized to ssh://",
			remoteAddr: "git@github.com:user/repo.git",
			expected:   "ssh://git@github.com/user/repo.git",
		},
		{
			name:       "SSH SCP custom user",
			remoteAddr: "myuser@example.com:path/to/repo.git",
			expected:   "ssh://myuser@example.com/path/to/repo.git",
		},
		{
			name:       "SSH SCP nested path",
			remoteAddr: "git@gitlab.com:group/subgroup/project.git",
			expected:   "ssh://git@gitlab.com/group/subgroup/project.git",
		},
		{
			name:       "SSH SCP IPv6 host",
			remoteAddr: "git@[2001:db8::1]:user/repo.git",
			expected:   "ssh://git@[2001:db8::1]/user/repo.git",
		},
		{
			name:       "ssh:// URL passed through",
			remoteAddr: "ssh://git@github.com/user/repo.git",
			expected:   "ssh://git@github.com/user/repo.git",
		},
		{
			name:       "ssh:// URL with port passed through",
			remoteAddr: "ssh://git@example.com:2222/user/repo.git",
			expected:   "ssh://git@example.com:2222/user/repo.git",
		},
		{
			name:        "SSH URL rejects username/password auth",
			remoteAddr:  "git@github.com:user/repo.git",
			authUser:    "user",
			authPass:    "pass",
			shouldError: true,
		},
		{
			name:       "HTTPS URL with auth gets credentials injected",
			remoteAddr: "https://github.com/user/repo.git",
			authUser:   "user",
			authPass:   "pass",
			expected:   "https://user:pass@github.com/user/repo.git",
		},
		{
			name:       "HTTPS URL without auth unchanged",
			remoteAddr: "https://github.com/user/repo.git",
			expected:   "https://github.com/user/repo.git",
		},
		{
			name:       "git:// URL with auth gets credentials injected",
			remoteAddr: "git://github.com/user/repo.git",
			authUser:   "user",
			authPass:   "pass",
			expected:   "git://user:pass@github.com/user/repo.git",
		},
		{
			name:       "Local path passed through unchanged",
			remoteAddr: "/srv/git/repo.git",
			expected:   "/srv/git/repo.git",
		},
		{
			// host:path without a user is not SCP syntax (ParseGitURL requires
			// "@"), so it is treated as a local path and left unchanged instead
			// of being silently rewritten to ssh://git@host/path
			name:       "host:path without user is not treated as SSH",
			remoteAddr: "github.com:user/repo.git",
			expected:   "github.com:user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRemoteAddr(tt.remoteAddr, tt.authUser, tt.authPass)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
